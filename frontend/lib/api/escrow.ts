import { apiClient } from './client';



// =============================================================================
// Multipart Upload — browser-direct large file uploads
// =============================================================================

interface InitMultipartResponse {
  uploadId: string;
  key: string;
  partSize: number;
}

interface PresignPartResponse {
  url: string;
}

interface CompletePart {
  partNumber: number;
  etag: string;
}

export interface UploadProgress {
  loaded: number;
  total: number;
  percentage: number;
  /** Currently uploading part numbers */
  activeParts: number[];
  /** Successfully completed part numbers */
  completedParts: number[];
}

/**
 * Upload a large file directly to S3/MinIO using presigned multipart upload.
 * Splits the file into chunks, uploads up to `concurrency` parts in parallel,
 * and reports progress via the `onProgress` callback.
 *
 * @returns The final S3 object key
 */
export async function uploadEscrowFile(
  file: File,
  workflowType: string,
  tld: string,
  options?: {
    onProgress?: (progress: UploadProgress) => void;
    abortSignal?: AbortSignal;
    concurrency?: number;
  }
): Promise<string> {
  const concurrency = options?.concurrency ?? 3;
  const signal = options?.abortSignal;

  // === Phase 1: Init multipart upload ===
  let initData: InitMultipartResponse;
  try {
    const resp = await apiClient.post<InitMultipartResponse>(
      '/escrow/uploads/multipart/init',
      { workflowType, tld, filename: file.name },
      { signal }
    );
    initData = resp.data;
  } catch (err: any) {
    const detail = err?.response?.data?.error || err?.message || 'Unknown error';
    throw new Error(
      `Failed to initialize upload: ${detail}\n\n` +
      `This usually means the API server cannot reach the storage backend (MinIO/S3). ` +
      `Check that MinIO is running and MINIO_ENDPOINT is correctly configured.`
    );
  }

  const { uploadId, key, partSize } = initData;
  const totalParts = Math.ceil(file.size / partSize);
  const completedParts: CompletePart[] = [];
  const completedPartNums: number[] = [];
  const activeParts: number[] = [];
  let totalUploaded = 0;

  const reportProgress = () => {
    options?.onProgress?.({
      loaded: totalUploaded,
      total: file.size,
      percentage: file.size > 0 ? Math.round((totalUploaded / file.size) * 100) : 0,
      activeParts: [...activeParts],
      completedParts: [...completedPartNums],
    });
  };

  try {
    // === Phase 2: Upload parts with bounded concurrency ===
    let nextPart = 1;

    const uploadPart = async (partNumber: number): Promise<void> => {
      if (signal?.aborted) throw new DOMException('Upload aborted', 'AbortError');

      const start = (partNumber - 1) * partSize;
      const end = Math.min(start + partSize, file.size);
      const chunk = file.slice(start, end);
      const chunkSize = end - start;

      // Get presigned URL for this part
      let presignUrl: string;
      try {
        const { data: presignData } = await apiClient.post<PresignPartResponse>(
          '/escrow/uploads/multipart/presign-part',
          { key, uploadId, partNumber },
          { signal }
        );
        presignUrl = presignData.url;
      } catch (err: any) {
        const detail = err?.response?.data?.error || err?.message || 'Unknown error';
        throw new Error(
          `Failed to get presigned URL for part ${partNumber}/${totalParts}: ${detail}`
        );
      }

      // Upload chunk directly to S3 via presigned URL
      activeParts.push(partNumber);
      reportProgress();

      const xhr = new XMLHttpRequest();
      const etag = await new Promise<string>((resolve, reject) => {
        xhr.open('PUT', presignUrl);

        xhr.upload.onprogress = (e) => {
          if (e.lengthComputable) {
            const otherPartsBytes = completedPartNums.reduce((sum, pn) => {
              const pStart = (pn - 1) * partSize;
              const pEnd = Math.min(pStart + partSize, file.size);
              return sum + (pEnd - pStart);
            }, 0);
            totalUploaded = otherPartsBytes + e.loaded;
            reportProgress();
          }
        };

        xhr.onload = () => {
          if (xhr.status >= 200 && xhr.status < 300) {
            const etag = xhr.getResponseHeader('ETag') || '';
            if (!etag) {
              // ETag missing — likely a CORS issue where the header isn't exposed
              reject(new Error(
                `Part ${partNumber}/${totalParts}: Upload succeeded (HTTP ${xhr.status}) but ETag header is missing.\n\n` +
                `This is usually a CORS configuration issue — the storage server needs to expose the ETag header. ` +
                `For MinIO, ensure MINIO_API_CORS_ALLOW_ORIGIN is set. For AWS S3, add ETag to ExposeHeaders in the bucket CORS policy.`
              ));
              return;
            }
            resolve(etag.replace(/"/g, ''));
          } else {
            // Try to parse S3 XML error response
            let s3Error = '';
            try {
              const parser = new DOMParser();
              const doc = parser.parseFromString(xhr.responseText, 'text/xml');
              const code = doc.querySelector('Code')?.textContent;
              const message = doc.querySelector('Message')?.textContent;
              if (code || message) s3Error = ` — S3: ${code}: ${message}`;
            } catch { /* not XML, use raw response */ }

            const hint = xhr.status === 403
              ? '\n\nHTTP 403 usually means the presigned URL has expired or the signature is invalid. Check that the API server and storage server clocks are in sync.'
              : xhr.status === 404
              ? '\n\nHTTP 404 means the upload was not found — it may have been aborted or timed out.'
              : '';

            reject(new Error(
              `Part ${partNumber}/${totalParts} upload failed: HTTP ${xhr.status}${s3Error}${hint}\n` +
              `Target: ${new URL(presignUrl).origin}${new URL(presignUrl).pathname}`
            ));
          }
        };

        xhr.onerror = () => {
          // XHR onerror fires for network-level failures (DNS, connection refused, CORS block)
          const targetOrigin = (() => { try { return new URL(presignUrl).origin; } catch { return presignUrl; } })();
          reject(new Error(
            `Part ${partNumber}/${totalParts}: Network error uploading to storage.\n\n` +
            `The browser could not reach ${targetOrigin}. Common causes:\n` +
            `• CORS: The storage server is not configured to accept cross-origin requests from this domain.\n` +
            `• Connectivity: The presigned URL points to a host the browser cannot reach (e.g., a Docker-internal hostname like "minio:9000").\n` +
            `• Firewall: The storage port is not exposed or is blocked.\n\n` +
            `Presigned URL: ${presignUrl}`
          ));
        };

        xhr.onabort = () => reject(new DOMException('Upload aborted', 'AbortError'));

        if (signal) {
          signal.addEventListener('abort', () => xhr.abort(), { once: true });
        }

        xhr.send(chunk);
      });

      // Mark part as completed
      const idx = activeParts.indexOf(partNumber);
      if (idx !== -1) activeParts.splice(idx, 1);
      completedParts.push({ partNumber, etag });
      completedPartNums.push(partNumber);

      totalUploaded = completedPartNums.reduce((sum, pn) => {
        const pStart = (pn - 1) * partSize;
        const pEnd = Math.min(pStart + partSize, file.size);
        return sum + (pEnd - pStart);
      }, 0);
      reportProgress();
    };

    // Run with bounded concurrency
    const workers = Array.from({ length: Math.min(concurrency, totalParts) }, async () => {
      while (nextPart <= totalParts) {
        const partNum = nextPart++;
        await uploadPart(partNum);
      }
    });

    await Promise.all(workers);

    // === Phase 3: Complete multipart upload ===
    try {
      completedParts.sort((a, b) => a.partNumber - b.partNumber);
      await apiClient.post('/escrow/uploads/multipart/complete', {
        key,
        uploadId,
        parts: completedParts,
      }, { signal });
    } catch (err: any) {
      const detail = err?.response?.data?.error || err?.message || 'Unknown error';
      throw new Error(
        `All ${totalParts} parts uploaded successfully, but failed to finalize: ${detail}\n\n` +
        `The parts are still on storage and may need manual cleanup. Upload ID: ${uploadId}`
      );
    }

    return key;
  } catch (err) {
    // Abort the multipart upload on any failure (except during finalize)
    try {
      await apiClient.post('/escrow/uploads/multipart/abort', { key, uploadId });
    } catch {
      // Best effort cleanup
    }
    throw err;
  }
}
