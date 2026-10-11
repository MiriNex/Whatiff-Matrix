import { computed, signal, Signal } from '@angular/core';

/** How many of a set of rows are selected; drives a select-all checkbox (`some` is indeterminate). */
export type SelectionState = 'none' | 'some' | 'all';

/**
 * Checkbox selection for a bulk-action table (Thread Manager, gallery list): a set of picked ids
 * plus the few operations a table and its bulk bar need. Owned by the service that holds the rows;
 * it knows nothing about which rows exist, so callers pass the ids that are shown.
 */
export class SelectionModel<K = string> {
  private readonly ids = signal<ReadonlySet<K>>(new Set());

  /** The picked ids. */
  readonly selectedIds: Signal<ReadonlySet<K>> = this.ids.asReadonly();
  readonly count = computed(() => this.ids().size);

  has(id: K): boolean {
    return this.ids().has(id);
  }

  toggle(id: K): void {
    const next = new Set(this.ids());
    if (!next.delete(id)) {
      next.add(id);
    }
    this.ids.set(next);
  }

  /** Picks (or unpicks) every one of `ids`, leaving the rest of the selection as it is. */
  setAll(ids: readonly K[], selected: boolean): void {
    const next = new Set(this.ids());
    for (const id of ids) {
      if (selected) {
        next.add(id);
      } else {
        next.delete(id);
      }
    }
    this.ids.set(next);
  }

  /** Replaces the selection with exactly `ids`. */
  replace(ids: Iterable<K>): void {
    this.ids.set(new Set(ids));
  }

  clear(): void {
    if (this.ids().size > 0) {
      this.ids.set(new Set());
    }
  }

  /** Drops picked ids that are no longer among `ids` (rows deleted or moved out of view). */
  retain(ids: Iterable<K>): void {
    const keep = new Set(ids);
    const current = this.ids();
    const next = [...current].filter(id => keep.has(id));
    if (next.length !== current.size) {
      this.ids.set(new Set(next));
    }
  }

  /** Whether none, some or all of `ids` (the rows shown) are picked. */
  stateOf(ids: readonly K[]): SelectionState {
    if (ids.length === 0) return 'none';
    const selected = this.ids();
    const picked = ids.reduce((count, id) => count + (selected.has(id) ? 1 : 0), 0);
    if (picked === 0) return 'none';
    return picked === ids.length ? 'all' : 'some';
  }
}
