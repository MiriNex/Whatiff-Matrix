import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';

import { DataTableComponent } from '../../../shared/ui/data-table/data-table.component';
import { SelectionState } from '../../../shared/ui/helpers/selection-model';
import { FolderTileVm } from '../helpers/gallery-folder.helpers';
import { GalleryTileVm } from '../helpers/gallery-vm.helpers';
import { GalleryListRowComponent } from './gallery-list-row.component';

/**
 * The gallery as a table (the Thread Manager's table layout, `ui-data-table`): folders one level
 * down, then the loaded files with a checkbox each for bulk move and delete. Load more appends to
 * the same table, and what is already picked stays picked.
 */
@Component({
  selector: 'app-gallery-list',
  standalone: true,
  imports: [DataTableComponent, GalleryListRowComponent],
  templateUrl: './gallery-list.component.html',
  styleUrl: './gallery-list.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class GalleryListComponent {
  readonly tiles = input<GalleryTileVm[]>([]);
  readonly folderTiles = input<FolderTileVm[]>([]);
  readonly selectedIds = input<ReadonlySet<string>>(new Set());
  readonly selectionState = input<SelectionState>('none');
  readonly loading = input(false);
  readonly loadingMore = input(false);
  readonly hasMore = input(false);
  readonly error = input<string | null>(null);
  readonly emptyMessage = input('No images match these filters yet.');

  readonly toggleSelect = output<string>();
  readonly toggleSelectAll = output<void>();
  readonly openImage = output<string>();
  readonly openFolder = output<string>();
  readonly deleteImage = output<string>();
  readonly retry = output<void>();
  readonly loadMore = output<void>();

  /** What the table shows instead of rows: loading, the error, or why it is empty. */
  readonly message = computed(() => {
    const empty = this.tiles().length === 0 && this.folderTiles().length === 0;
    if (this.error()) return this.error();
    if (this.loading() && empty) return 'Loading the gallery…';
    return empty ? this.emptyMessage() : null;
  });
}
