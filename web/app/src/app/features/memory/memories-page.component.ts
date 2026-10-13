import { ChangeDetectionStrategy, Component, OnInit, inject, signal, viewChild } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { firstValueFrom } from 'rxjs';

import { MemoriesListTabComponent } from './memories-list-tab.component';
import { MemoryMergeHistoryPageComponent } from './memory-merge-history-page.component';
import { CompactionLogPageComponent } from './compaction-log-page.component';
import { HelpHintComponent } from '../../shared/ui/help-hint/help-hint.component';
import { TooltipDirective } from '../../shared/ui/tooltip/tooltip.directive';
import { ModalComponent } from '../../shared/ui/modal/modal.component';
import { ButtonComponent } from '../../shared/ui/button/button.component';
import { MemoryService } from '../../core/services/memory.service';
import { PersonalityService } from '../../core/services/personality.service';
import { apiErrorMessage } from '../../core/utils/api-error.helpers';

export type MemoriesPageTab = 'memories' | 'merge-history' | 'compaction-log';

interface PersonalityOption {
  id: string;
  name: string;
}

@Component({
  selector: 'app-memories-page',
  standalone: true,
  imports: [MemoriesListTabComponent, MemoryMergeHistoryPageComponent, CompactionLogPageComponent, HelpHintComponent, TooltipDirective, ModalComponent, ButtonComponent],
  templateUrl: './memories-page.component.html',
  styleUrl: './memories-page.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class MemoriesPageComponent implements OnInit {
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly memoryService = inject(MemoryService);
  private readonly personalityService = inject(PersonalityService);

  readonly listTab = viewChild(MemoriesListTabComponent);

  readonly activeTab = signal<MemoriesPageTab>('memories');

  // Create-memory modal state. The ?create=1 query param opens the modal on load
  // (same convention as /personality?create=1) and is then cleared from the URL.
  readonly createOpen = signal(false);
  readonly createContent = signal('');
  readonly createPinnedId = signal('');
  readonly createSaving = signal(false);
  readonly createError = signal<string | null>(null);
  readonly personalities = signal<PersonalityOption[]>([]);

  ngOnInit(): void {
    this.route.queryParams.subscribe(params => {
      this.activeTab.set(normalizeTab(params['tab']));
      if (params['create'] === '1') {
        this.openCreateModal();
        void this.router.navigate([], {
          relativeTo: this.route,
          queryParams: { create: null },
          queryParamsHandling: 'merge',
          replaceUrl: true,
        });
      }
    });

    this.personalityService.listPersonalities(1, 200).subscribe({
      next: result => {
        this.personalities.set((result.results ?? []).map(item => ({ id: item.id, name: item.name })));
      },
    });
  }

  setActiveTab(tab: MemoriesPageTab): void {
    this.activeTab.set(tab);
    void this.router.navigate([], {
      relativeTo: this.route,
      queryParams: tab === 'memories' ? { tab: null } : { tab },
      queryParamsHandling: 'merge',
      replaceUrl: true,
    });
  }

  openCreateModal(): void {
    this.createContent.set('');
    this.createPinnedId.set('');
    this.createError.set(null);
    this.createOpen.set(true);
  }

  closeCreateModal(): void {
    this.createOpen.set(false);
  }

  async saveNewMemory(): Promise<void> {
    const content = this.createContent().trim();
    if (!content || this.createSaving()) return;
    this.createSaving.set(true);
    this.createError.set(null);
    try {
      const pinned = this.createPinnedId();
      await firstValueFrom(this.memoryService.createMemory({
        content,
        level: 'global',
        type: 'Context',
        pinned_personality_id: pinned || null,
      }));
      this.createOpen.set(false);
      this.listTab()?.refresh();
    } catch (error) {
      this.createError.set(apiErrorMessage(error, 'Failed to save memory'));
    } finally {
      this.createSaving.set(false);
    }
  }
}

function normalizeTab(raw: unknown): MemoriesPageTab {
  if (raw === 'merge-history' || raw === 'compaction-log') return raw;
  return 'memories';
}
