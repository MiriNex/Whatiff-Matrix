/** What a batch run by {@link settleWithLimit} did: which items went through and which failed, and why. */
export interface SettledBatch<T> {
  succeeded: T[];
  failed: { item: T; error: unknown }[];
}

/**
 * Runs `worker` over `items` with at most `limit` running at once, and waits for every one to
 * settle. A failure does not stop the rest; it is reported per item, so a bulk action can say
 * exactly which items it could not do. Results keep the input order.
 */
export async function settleWithLimit<T>(
  items: readonly T[],
  limit: number,
  worker: (item: T) => Promise<unknown>,
): Promise<SettledBatch<T>> {
  const outcomes: ({ ok: true } | { ok: false; error: unknown })[] = new Array(items.length);
  let next = 0;
  const lane = async (): Promise<void> => {
    while (next < items.length) {
      const index = next++;
      try {
        await worker(items[index]);
        outcomes[index] = { ok: true };
      } catch (error) {
        outcomes[index] = { ok: false, error };
      }
    }
  };
  const lanes = Math.max(1, Math.min(Math.floor(limit) || 1, items.length));
  await Promise.all(Array.from({ length: lanes }, lane));

  const batch: SettledBatch<T> = { succeeded: [], failed: [] };
  items.forEach((item, index) => {
    const outcome = outcomes[index];
    if (outcome.ok) {
      batch.succeeded.push(item);
    } else {
      batch.failed.push({ item, error: outcome.error });
    }
  });
  return batch;
}
