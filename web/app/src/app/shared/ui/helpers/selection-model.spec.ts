import { SelectionModel } from './selection-model';

describe('SelectionModel', () => {
  let selection: SelectionModel;

  beforeEach(() => {
    selection = new SelectionModel();
  });

  it('toggles single ids and counts them', () => {
    selection.toggle('a');
    selection.toggle('b');
    expect(selection.count()).toBe(2);
    expect(selection.has('a')).toBe(true);

    selection.toggle('a');
    expect(selection.has('a')).toBe(false);
    expect(selection.count()).toBe(1);
  });

  it('selects and unselects a group without touching ids outside it', () => {
    selection.toggle('outside');
    selection.setAll(['a', 'b'], true);
    expect([...selection.selectedIds()].sort()).toEqual(['a', 'b', 'outside']);

    selection.setAll(['a', 'b'], false);
    expect([...selection.selectedIds()]).toEqual(['outside']);
  });

  it('reports none, some or all of the shown ids', () => {
    expect(selection.stateOf([])).toBe('none');
    expect(selection.stateOf(['a', 'b'])).toBe('none');
    selection.toggle('a');
    expect(selection.stateOf(['a', 'b'])).toBe('some');
    selection.toggle('b');
    expect(selection.stateOf(['a', 'b'])).toBe('all');
    // Picked ids that are not shown do not make a shown set "some".
    expect(selection.stateOf(['c'])).toBe('none');
  });

  it('retains only ids that still exist, and keeps the same set when nothing dropped', () => {
    selection.replace(['a', 'b', 'c']);
    const before = selection.selectedIds();
    selection.retain(['a', 'b', 'c', 'd']);
    expect(selection.selectedIds()).toBe(before);

    selection.retain(['b']);
    expect([...selection.selectedIds()]).toEqual(['b']);
  });

  it('clears, without emitting a new set when already empty', () => {
    const empty = selection.selectedIds();
    selection.clear();
    expect(selection.selectedIds()).toBe(empty);

    selection.replace(['a']);
    selection.clear();
    expect(selection.count()).toBe(0);
  });
});
