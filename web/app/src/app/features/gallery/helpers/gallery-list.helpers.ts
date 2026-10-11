/** How the gallery shows its files: thumbnails in a grid, or rows in a table with bulk actions. */
export type GalleryLayout = 'grid' | 'list';

/** localStorage key for the last layout chosen in this browser. */
export const GALLERY_LAYOUT_KEY = 'gallery_layout';

/** The layout chosen last time, or the grid when there is none (or storage is unavailable). */
export function readGalleryLayout(): GalleryLayout {
  try {
    return localStorage.getItem(GALLERY_LAYOUT_KEY) === 'list' ? 'list' : 'grid';
  } catch {
    return 'grid';
  }
}

export function writeGalleryLayout(layout: GalleryLayout): void {
  try {
    localStorage.setItem(GALLERY_LAYOUT_KEY, layout);
  } catch {
    // Private mode or a full quota: the choice just does not survive a reload.
  }
}

/** What to tell the user after a bulk delete, naming what could not be deleted. */
export function describeBulkDelete(deleted: number, failedNames: readonly string[]): string {
  const total = deleted + failedNames.length;
  const noun = (count: number) => (count === 1 ? 'item' : 'items');
  if (failedNames.length === 0) {
    return `Deleted ${deleted} ${noun(deleted)}.`;
  }
  const shown = failedNames.slice(0, 3).map(name => `“${name}”`).join(', ');
  const more = failedNames.length > 3 ? ` and ${failedNames.length - 3} more` : '';
  return `Deleted ${deleted} of ${total} ${noun(total)}. Could not delete ${shown}${more}; they are still selected so you can try again.`;
}
