import { isAxiosError } from 'axios';
import { apiClient } from './client';
import {
  fallbackSyntheticFilename,
  filenameFromContentDisposition,
  type SyntheticDepositParams,
} from '@/lib/escrow/synthetic';

export interface SyntheticDeposit {
  blob: Blob;
  filename: string;
  /** The seed it was generated from, to reproduce it; null if the header was not readable. */
  seed: string | null;
}

/**
 * Generates a synthetic RDE deposit. The server streams gzip-compressed XML;
 * the whole file is held in memory as a Blob until the caller saves it.
 */
export async function generateSyntheticDeposit(params: SyntheticDepositParams, signal?: AbortSignal): Promise<SyntheticDeposit> {
  try {
    const resp = await apiClient.post<Blob>('/escrow/synthetic', params, { responseType: 'blob', signal });
    const header = (name: string): string | null => {
      const v = resp.headers?.[name];
      return typeof v === 'string' ? v : null;
    };
    return {
      blob: resp.data,
      filename: filenameFromContentDisposition(header('content-disposition')) ?? fallbackSyntheticFilename(params.tld),
      seed: header('x-synthetic-seed'),
    };
  } catch (err) {
    throw new Error(await syntheticErrorMessage(err));
  }
}

/**
 * With responseType 'blob' an error body arrives as a Blob too, so the
 * server's {"error": "..."} has to be read out of it.
 */
async function syntheticErrorMessage(err: unknown): Promise<string> {
  if (isAxiosError(err)) {
    const data: unknown = err.response?.data;
    if (isBlobLike(data)) {
      try {
        const parsed = JSON.parse(await readBlobText(data));
        if (parsed && typeof parsed.error === 'string') return parsed.error;
      } catch {
        // not JSON; fall through
      }
    } else if (data && typeof data === 'object' && typeof (data as { error?: unknown }).error === 'string') {
      return (data as { error: string }).error;
    }
    if (err.response) return `The server refused the request (HTTP ${err.response.status}).`;
    return err.message || 'Could not reach the API.';
  }
  return err instanceof Error ? err.message : 'Could not generate the deposit.';
}

// Duck-typed rather than instanceof: a Blob from another realm (a test DOM, an
// iframe) is still a Blob.
function isBlobLike(v: unknown): v is Blob {
  return typeof v === 'object' && v !== null && typeof (v as Blob).size === 'number' && typeof (v as Blob).slice === 'function';
}

function readBlobText(blob: Blob): Promise<string> {
  if (typeof blob.text === 'function') return blob.text();
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result ?? ''));
    reader.onerror = () => reject(reader.error);
    reader.readAsText(blob);
  });
}
