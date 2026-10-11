import { chunk } from './chunk.helpers';

describe('chunk', () => {
  it('splits into runs of at most the size, in order', () => {
    expect(chunk([1, 2, 3, 4, 5], 2)).toEqual([[1, 2], [3, 4], [5]]);
    expect(chunk([1, 2, 3], 3)).toEqual([[1, 2, 3]]);
  });

  it('returns no runs for nothing, and treats a nonsense size as one', () => {
    expect(chunk([], 500)).toEqual([]);
    expect(chunk(['a', 'b'], 0)).toEqual([['a'], ['b']]);
  });
});
