import { resolveAuthToken } from './client';
import { getApiUrl } from '@/lib/env';

// ---------------------------------------------------------------------------
// Types matching the Go Result struct
// ---------------------------------------------------------------------------

export type Outcome = 'answer' | 'escalate' | 'action_required';

export interface Evidence {
  tool: string;
  input: any;
  result: any;
}

export interface AgentResult {
  outcome: Outcome;
  answer?: string;
  reason?: string;
  action?: string;
  evidence: Evidence[];
  iterations: number;
  total_usage: {
    input_tokens: number;
    output_tokens: number;
  };
}

// ---------------------------------------------------------------------------
// SSE event types
// ---------------------------------------------------------------------------

export interface AgentSSEEvent {
  type: 'result' | 'error';
  data: AgentResult | { error: string };
}

// ---------------------------------------------------------------------------
// SSE streaming client
// ---------------------------------------------------------------------------

/**
 * Send a question to the agent endpoint and stream SSE events back.
 *
 * Uses native `fetch` (not axios) because we need to read the response as a
 * streaming ReadableStream for SSE parsing. The base URL and auth token are
 * resolved through the same helpers the apiClient interceptor uses, so all
 * requests go through the same origin with the same credentials.
 */
export async function askAgent(
  question: string,
  onEvent: (event: AgentSSEEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  const baseURL = getApiUrl();
  const token = resolveAuthToken();

  const response = await fetch(`${baseURL}/agent/ask`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Accept': 'text/event-stream',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({ question }),
    signal,
  });

  if (!response.ok) {
    const errorText = await response.text().catch(() => response.statusText);
    onEvent({
      type: 'error',
      data: { error: `Agent request failed (${response.status}): ${errorText}` },
    });
    return;
  }

  const reader = response.body?.getReader();
  if (!reader) {
    onEvent({ type: 'error', data: { error: 'No response stream available' } });
    return;
  }

  const decoder = new TextDecoder();
  let buffer = '';

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });

      // SSE format: "event: <type>\ndata: <json>\n\n"
      const parts = buffer.split('\n\n');
      // Keep the last (possibly incomplete) chunk in the buffer
      buffer = parts.pop() || '';

      for (const part of parts) {
        if (!part.trim()) continue;

        let eventType = 'result';
        let eventData = '';

        for (const line of part.split('\n')) {
          if (line.startsWith('event:')) {
            eventType = line.slice(6).trim();
          } else if (line.startsWith('data:')) {
            eventData = line.slice(5).trim();
          }
        }

        if (!eventData) continue;

        try {
          const parsed = JSON.parse(eventData);
          onEvent({ type: eventType as AgentSSEEvent['type'], data: parsed });
        } catch {
          onEvent({ type: 'error', data: { error: `Failed to parse SSE data: ${eventData}` } });
        }
      }
    }
  } finally {
    reader.releaseLock();
  }
}
