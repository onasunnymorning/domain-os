import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useFreshRead } from '../useFreshRead';

describe('useFreshRead', () => {
  afterEach(() => vi.useRealTimers());

  it('does not mount on a cached copy, only on a fetch completed after mount', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-11-02T12:00:00Z'));
    const cached = { data: { x: 1 }, dataUpdatedAt: Date.now() - 60_000, isFetching: false };
    const { result, rerender } = renderHook((q: Parameters<typeof useFreshRead>[0]) => useFreshRead(q), { initialProps: cached });
    expect(result.current.fetchedAt).toBeNull();
    rerender({ ...cached, isFetching: true });
    expect(result.current.fetchedAt).toBeNull();
    vi.setSystemTime(new Date('2026-11-02T12:00:02Z'));
    rerender({ data: { x: 2 }, dataUpdatedAt: Date.now(), isFetching: false });
    expect(result.current.fetchedAt).toBe(Date.now());
  });

  it('stays mounted through a later failed refetch, and blocks only on an initial error', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-11-02T12:00:00Z'));
    const { result, rerender } = renderHook((q: Parameters<typeof useFreshRead>[0]) => useFreshRead(q), {
      initialProps: { data: undefined, dataUpdatedAt: 0, isFetching: true } as Parameters<typeof useFreshRead>[0],
    });
    vi.setSystemTime(new Date('2026-11-02T12:00:01Z'));
    rerender({ data: { x: 1 }, dataUpdatedAt: Date.now(), isFetching: false });
    const at = result.current.fetchedAt;
    expect(at).not.toBeNull();
    rerender({ data: { x: 1 }, dataUpdatedAt: at!, isFetching: false, error: new Error('offline') });
    expect(result.current.fetchedAt).toBe(at);
    expect(result.current.initialError).toBeUndefined();

    const failed = renderHook(() => useFreshRead({ data: undefined, dataUpdatedAt: 0, isFetching: false, error: new Error('403') }));
    expect(failed.result.current.fetchedAt).toBeNull();
    expect(failed.result.current.initialError).toBeInstanceOf(Error);
  });
});
