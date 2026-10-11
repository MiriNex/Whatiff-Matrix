import { settleWithLimit } from './bounded-concurrency.helpers';

function deferred(): { promise: Promise<void>; resolve: () => void; reject: (error: unknown) => void } {
  let resolve!: () => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<void>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

describe('settleWithLimit', () => {
  it('never runs more than the limit at once', async () => {
    let running = 0;
    let peak = 0;
    const result = await settleWithLimit([1, 2, 3, 4, 5, 6, 7], 3, async () => {
      running++;
      peak = Math.max(peak, running);
      await new Promise(resolve => setTimeout(resolve, 0));
      running--;
    });
    expect(peak).toBe(3);
    expect(result.succeeded).toEqual([1, 2, 3, 4, 5, 6, 7]);
    expect(result.failed).toEqual([]);
  });

  it('keeps going past failures and reports each one with its error, in input order', async () => {
    const pending = new Map([
      ['a', deferred()],
      ['b', deferred()],
      ['c', deferred()],
    ]);
    const run = settleWithLimit(['a', 'b', 'c'], 2, item => pending.get(item)!.promise);
    pending.get('b')!.reject(new Error('b failed'));
    pending.get('a')!.resolve();
    await Promise.resolve();
    pending.get('c')!.resolve();

    const result = await run;
    expect(result.succeeded).toEqual(['a', 'c']);
    expect(result.failed).toHaveLength(1);
    expect(result.failed[0].item).toBe('b');
    expect((result.failed[0].error as Error).message).toBe('b failed');
  });

  it('handles an empty batch and a nonsense limit', async () => {
    expect(await settleWithLimit([], 4, async () => undefined)).toEqual({ succeeded: [], failed: [] });
    const worker = vi.fn().mockResolvedValue(undefined);
    const result = await settleWithLimit(['x', 'y'], 0, worker);
    expect(worker).toHaveBeenCalledTimes(2);
    expect(result.succeeded).toEqual(['x', 'y']);
  });
});
