import type { MockedObject } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { provideZonelessChangeDetection } from '@angular/core';
import { of, throwError } from 'rxjs';

import { FileAttachment } from '../models/file-attachment.model';
import { PaginatedResponse } from '../models/common.model';
import { GALLERY_BATCH_LIMIT, ImageGalleryService } from './image-gallery.service';
import { GalleryViewService } from './gallery-view.service';
import { GALLERY_LAYOUT_KEY } from '../../features/gallery/helpers/gallery-list.helpers';

function file(id: string, fileType = 'image/png'): FileAttachment {
  return { id, user_id: 'u', name: `${id}.png`, file_type: fileType, created_at: '2026-05-01T00:00:00Z' };
}

function page(rows: FileAttachment[], total = rows.length): PaginatedResponse<FileAttachment> {
  return { results: rows, total_count: total, page: 1 } as PaginatedResponse<FileAttachment>;
}

describe('GalleryViewService list view and bulk actions', () => {
  let service: GalleryViewService;
  let api: MockedObject<Pick<ImageGalleryService, 'listImages' | 'listFolders' | 'deleteFiles' | 'moveImages'>>;

  const create = () => {
    TestBed.configureTestingModule({
      providers: [provideZonelessChangeDetection(), GalleryViewService, { provide: ImageGalleryService, useValue: api }],
    });
    service = TestBed.inject(GalleryViewService);
    service.loadInitial();
  };

  beforeEach(() => {
    localStorage.removeItem(GALLERY_LAYOUT_KEY);
    api = {
      listImages: vi.fn().mockReturnValue(of(page([file('a'), file('b'), file('c')], 5))),
      listFolders: vi.fn().mockReturnValue(of([])),
      // The server deletes everything it is sent unless a test says otherwise.
      deleteFiles: vi.fn((ids: readonly string[]) => of({ deleted: [...ids], failed: [] })),
      moveImages: vi.fn((ids: readonly string[]) => of(ids.length)),
    } as unknown as typeof api;
  });

  afterEach(() => localStorage.removeItem(GALLERY_LAYOUT_KEY));

  describe('layout', () => {
    it('starts on the grid, and remembers the list for next time', () => {
      create();
      expect(service.layout()).toBe('grid');

      service.setLayout('list');
      expect(service.layout()).toBe('list');
      expect(localStorage.getItem(GALLERY_LAYOUT_KEY)).toBe('list');

      TestBed.resetTestingModule();
      create();
      expect(service.layout()).toBe('list');
    });

    it('starts fresh when switching: no selection, no grid select mode', () => {
      create();
      service.setSelectionMode(true);
      service.toggleSelected('a');

      service.setLayout('list');

      expect(service.selectedCount()).toBe(0);
      expect(service.selectionMode()).toBe(false);
    });
  });

  describe('selection across filters and pages', () => {
    beforeEach(() => create());

    it('keeps picks when more is loaded, and select all covers what is loaded', () => {
      service.toggleSelected('a');
      api.listImages.mockReturnValue(of(page([file('d'), file('e')], 5)));

      service.loadNextPage();

      expect([...service.selectedIds()]).toEqual(['a']);
      service.selectAllShown();
      expect(service.selectedCount()).toBe(5);
      expect(service.selectionState()).toBe('all');
    });

    it('clears the selection when a filter changes, and keeps it when nothing did', () => {
      service.toggleSelected('a');
      service.setFilters({ query: '' });
      expect(service.selectedCount()).toBe(1);

      service.setFilters({ source: 'uploaded' });
      expect(service.selectedCount()).toBe(0);

      service.toggleSelected('a');
      service.setSelectedPersonalityIds(['p1']);
      expect(service.selectedCount()).toBe(0);
    });

    it('clears the selection when the folder or kind changes', () => {
      service.toggleSelected('a');
      service.openFolder('charts');
      expect(service.selectedCount()).toBe(0);

      service.toggleSelected('a');
      service.setKind('files');
      expect(service.selectedCount()).toBe(0);
    });

    it('drops picks a reload no longer shows', () => {
      service.toggleSelected('a');
      service.toggleSelected('c');
      api.listImages.mockReturnValue(of(page([file('a'), file('b')])));

      service.refresh();

      expect([...service.selectedIds()]).toEqual(['a']);
    });
  });

  describe('deleteFiles', () => {
    beforeEach(() => create());

    it('deletes through the bulk route, removes what went, and reports the count', async () => {
      service.selectAllShown();

      const deleted = await service.deleteFiles(['a', 'b']);

      expect(deleted).toBe(2);
      expect(api.deleteFiles).toHaveBeenCalledTimes(1);
      expect(api.deleteFiles).toHaveBeenCalledWith(['a', 'b']);
      expect(service.images().map(row => row.id)).toEqual(['c']);
      expect(service.totalCount()).toBe(3);
      expect([...service.selectedIds()]).toEqual(['c']);
      expect(service.bulkResult()).toEqual({ text: 'Deleted 2 items.', failed: false });
      expect(api.listFolders).toHaveBeenCalled();
    });

    it('keeps what the server could not delete shown and selected, naming it', async () => {
      api.deleteFiles.mockReturnValue(of({ deleted: ['a', 'c'], failed: [{ id: 'b', error: 'could not delete the stored image' }] }));
      service.selectAllShown();

      const deleted = await service.deleteFiles(['a', 'b', 'c']);

      expect(deleted).toBe(2);
      expect(service.images().map(row => row.id)).toEqual(['b']);
      expect([...service.selectedIds()]).toEqual(['b']);
      expect(service.bulkResult()?.failed).toBe(true);
      expect(service.bulkResult()?.text).toContain('Could not delete “b.png”');
    });

    it('sends more than the server cap in chunks, and a failed chunk fails only its own files', async () => {
      const rows = Array.from({ length: GALLERY_BATCH_LIMIT + 20 }, (_, i) => file(`f${i}`));
      api.listImages.mockReturnValue(of(page(rows)));
      service.refresh();
      api.deleteFiles.mockImplementation(ids =>
        ids.length === 20 ? throwError(() => new Error('offline')) : of({ deleted: [...ids], failed: [] }),
      );
      service.selectAllShown();

      const deleted = await service.deleteFiles(rows.map(row => row.id));

      expect(api.deleteFiles.mock.calls.map(([ids]) => ids.length)).toEqual([GALLERY_BATCH_LIMIT, 20]);
      expect(deleted).toBe(GALLERY_BATCH_LIMIT);
      expect(service.images()).toHaveLength(20);
      expect(service.selectedCount()).toBe(20);
      expect(service.bulkResult()?.text).toContain(`Deleted ${GALLERY_BATCH_LIMIT} of ${GALLERY_BATCH_LIMIT + 20} items`);
    });

    it('ignores ids that are not loaded (never deletes what is not on screen)', async () => {
      const deleted = await service.deleteFiles(['zzz']);
      expect(deleted).toBe(0);
      expect(api.deleteFiles).not.toHaveBeenCalled();
    });
  });

  describe('moveImages', () => {
    beforeEach(() => create());

    it('sends more than the server cap in chunks', async () => {
      const rows = Array.from({ length: GALLERY_BATCH_LIMIT * 2 + 1 }, (_, i) => file(`m${i}`));
      api.listImages.mockReturnValue(of(page(rows)));
      service.refresh();
      service.selectAllShown();

      const ok = await service.moveImages(rows.map(row => row.id), 'archive');

      expect(ok).toBe(true);
      expect(api.moveImages.mock.calls.map(([ids, folder]) => [ids.length, folder])).toEqual([
        [GALLERY_BATCH_LIMIT, 'archive'],
        [GALLERY_BATCH_LIMIT, 'archive'],
        [1, 'archive'],
      ]);
      expect(service.images()).toEqual([]);
      expect(service.selectedCount()).toBe(0);
    });

    it('stops at a failed chunk, keeping what already moved and the rest still selected', async () => {
      const rows = Array.from({ length: GALLERY_BATCH_LIMIT + 3 }, (_, i) => file(`m${i}`));
      api.listImages.mockReturnValue(of(page(rows)));
      service.refresh();
      service.selectAllShown();
      api.moveImages.mockImplementation(ids =>
        ids.length === 3 ? throwError(() => ({ error: { error: 'Too many files in one move' } })) : of(ids.length),
      );

      const ok = await service.moveImages(rows.map(row => row.id), 'archive');

      expect(ok).toBe(false);
      expect(service.images().map(row => row.id)).toEqual(['m500', 'm501', 'm502']);
      expect([...service.selectedIds()]).toEqual(['m500', 'm501', 'm502']);
      expect(service.folderError()).toBe('Too many files in one move');
      expect(service.lastMove()?.ids.size).toBe(GALLERY_BATCH_LIMIT);
    });
  });
});
