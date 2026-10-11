import { describeBulkDelete, GALLERY_LAYOUT_KEY, readGalleryLayout, writeGalleryLayout } from './gallery-list.helpers';

describe('gallery list helpers', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    localStorage.removeItem(GALLERY_LAYOUT_KEY);
  });

  describe('layout preference', () => {
    it('round-trips the list, and falls back to the grid for anything else', () => {
      expect(readGalleryLayout()).toBe('grid');
      writeGalleryLayout('list');
      expect(readGalleryLayout()).toBe('list');
      localStorage.setItem(GALLERY_LAYOUT_KEY, 'tiles');
      expect(readGalleryLayout()).toBe('grid');
    });

    it('survives storage that throws', () => {
      vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
        throw new Error('denied');
      });
      vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
        throw new Error('quota');
      });
      expect(() => writeGalleryLayout('list')).not.toThrow();
      expect(readGalleryLayout()).toBe('grid');
    });
  });

  describe('describeBulkDelete', () => {
    it('counts what went', () => {
      expect(describeBulkDelete(1, [])).toBe('Deleted 1 item.');
      expect(describeBulkDelete(4, [])).toBe('Deleted 4 items.');
    });

    it('names what failed, a few at most, and says they stay selected', () => {
      expect(describeBulkDelete(1, ['a.png'])).toBe(
        'Deleted 1 of 2 items. Could not delete “a.png”; they are still selected so you can try again.',
      );
      expect(describeBulkDelete(0, ['a', 'b', 'c', 'd', 'e'])).toBe(
        'Deleted 0 of 5 items. Could not delete “a”, “b”, “c” and 2 more; they are still selected so you can try again.',
      );
    });
  });
});
