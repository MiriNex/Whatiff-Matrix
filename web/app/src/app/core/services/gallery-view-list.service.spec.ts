import type { MockedObject } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { provideZonelessChangeDetection } from '@angular/core';
import { Observable, of, Subject, throwError } from 'rxjs';

import { FileAttachment } from '../models/file-attachment.model';
import { PaginatedResponse } from '../models/common.model';
import { ImageGalleryService } from './image-gallery.service';
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
  let api: MockedObject<Pick<ImageGalleryService, 'listImages' | 'listFolders' | 'deleteFile'>>;

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
      deleteFile: vi.fn().mockReturnValue(of(undefined)),
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

    it('deletes each file through the gallery, removes it, and reports the count', async () => {
      service.selectAllShown();

      const deleted = await service.deleteFiles(['a', 'b']);

      expect(deleted).toBe(2);
      expect(api.deleteFile.mock.calls.map(([row]) => row.id)).toEqual(['a', 'b']);
      expect(service.images().map(row => row.id)).toEqual(['c']);
      expect(service.totalCount()).toBe(3);
      expect([...service.selectedIds()]).toEqual(['c']);
      expect(service.bulkResult()).toEqual({ text: 'Deleted 2 items.', failed: false });
      expect(api.listFolders).toHaveBeenCalled();
    });

    it('keeps going after a failure, leaving the failed file shown and selected', async () => {
      api.deleteFile.mockImplementation(row => (row.id === 'b' ? throwError(() => new Error('nope')) : of(undefined)));
      service.selectAllShown();

      const deleted = await service.deleteFiles(['a', 'b', 'c']);

      expect(deleted).toBe(2);
      expect(service.images().map(row => row.id)).toEqual(['b']);
      expect([...service.selectedIds()]).toEqual(['b']);
      expect(service.bulkResult()?.failed).toBe(true);
      expect(service.bulkResult()?.text).toContain('Could not delete “b.png”');
    });

    it('never has more than a few deletes in flight', async () => {
      const rows = Array.from({ length: 10 }, (_, i) => file(`f${i}`));
      api.listImages.mockReturnValue(of(page(rows)));
      service.refresh();
      const pending: Subject<void>[] = [];
      api.deleteFile.mockImplementation(() => {
        const done = new Subject<void>();
        pending.push(done);
        return done as Observable<void>;
      });

      const settle = () => new Promise(resolve => setTimeout(resolve, 0));
      const run = service.deleteFiles(rows.map(row => row.id));
      await settle();
      expect(pending).toHaveLength(4);

      for (let finished = 0; finished < 10; finished++) {
        expect(pending.length - finished).toBeLessThanOrEqual(4);
        pending[finished].next();
        pending[finished].complete();
        await settle();
      }
      expect(await run).toBe(10);
    });

    it('ignores ids that are not loaded (never deletes what is not on screen)', async () => {
      const deleted = await service.deleteFiles(['zzz']);
      expect(deleted).toBe(0);
      expect(api.deleteFile).not.toHaveBeenCalled();
    });
  });
});
