import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  inject,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Router } from '@angular/router';
import { AsyncPipe } from '@angular/common';
import { forkJoin, of } from 'rxjs';
import { catchError, finalize } from 'rxjs/operators';

import { Personality } from '../../core/models/personality.model';
import { PersonalityService } from '../../core/services/personality.service';
import { ChatService } from '../../core/services/chat.service';
import { FileAttachmentService } from '../../core/services/file-attachment.service';
import { AuthImagePipe } from '../../core/pipes/auth-image.pipe';
import {
  ChatIconComponent,
  FileIconComponent,
  LayersIconComponent,
} from '../../shared/ui/icons/icons';

interface ProjectCard {
  personality: Personality;
  docCount: number;
  threadCount: number;
}

/**
 * Projects hub. Each oracle is a workspace: its Construct documents plus its
 * chats, grouped in one place (Claude Projects / Mistral libraries style).
 */
@Component({
  selector: 'app-projects-page',
  imports: [AsyncPipe, AuthImagePipe, ChatIconComponent, FileIconComponent, LayersIconComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="projects">
      <header class="projects__header">
        <h1 class="projects__title">Projects</h1>
        <p class="projects__subtitle">
          Workspaces with documents and chats, one per oracle. Open one to manage its Construct and talk.
        </p>
      </header>

      @if (loading()) {
        <p class="projects__status">Loading projects…</p>
      } @else if (projects().length === 0) {
        <p class="projects__status">No projects yet. Create an oracle and it becomes a project here.</p>
      } @else {
        <ul class="projects__grid">
          @for (card of projects(); track card.personality.id) {
            <li>
              <button type="button" class="projects__card" (click)="openProject(card)">
                <span class="projects__avatar">
                  @if (card.personality.cover_image_url; as cover) {
                    <img [src]="cover | authImage | async" alt="" />
                  } @else {
                    <ui-layers-icon [size]="22" />
                  }
                </span>
                <span class="projects__meta">
                  <span class="projects__name">{{ card.personality.name }}</span>
                  <span class="projects__counts">
                    <span class="projects__count"><ui-file-icon [size]="12" /> {{ card.docCount }} doc{{ card.docCount === 1 ? '' : 's' }}</span>
                    <span class="projects__count"><ui-chat-icon [size]="12" /> {{ card.threadCount }} chat{{ card.threadCount === 1 ? '' : 's' }}</span>
                  </span>
                </span>
              </button>
            </li>
          }
        </ul>
      }
    </div>
  `,
  styles: `
    .projects {
      display: flex;
      flex-direction: column;
      gap: 1.25rem;
      margin: 0 auto;
      max-width: 56rem;
      padding: 1.5rem 1rem 3rem;
    }

    .projects__header {
      display: flex;
      flex-direction: column;
      gap: 0.25rem;
    }

    .projects__title {
      color: var(--color-text-primary);
      font-size: 1.375rem;
      font-weight: 600;
      margin: 0;
    }

    .projects__subtitle {
      color: var(--color-text-muted);
      font-size: 0.8125rem;
      margin: 0;
    }

    .projects__status {
      color: var(--color-text-muted);
      font-size: 0.875rem;
      padding: 2rem 0;
      text-align: center;
    }

    .projects__grid {
      display: grid;
      gap: 0.75rem;
      grid-template-columns: repeat(auto-fill, minmax(15rem, 1fr));
      list-style: none;
      margin: 0;
      padding: 0;
    }

    .projects__card {
      align-items: center;
      background: var(--color-surface-base);
      border: 1px solid var(--color-border-base);
      border-radius: 0.75rem;
      cursor: pointer;
      display: flex;
      gap: 0.75rem;
      padding: 0.875rem;
      text-align: left;
      width: 100%;
    }

    .projects__card:hover {
      background: var(--color-surface-elevated);
    }

    .projects__avatar {
      align-items: center;
      background: var(--color-surface-muted);
      border-radius: 0.625rem;
      display: flex;
      flex-shrink: 0;
      height: 2.75rem;
      justify-content: center;
      overflow: hidden;
      width: 2.75rem;
    }

    .projects__avatar img {
      height: 100%;
      object-fit: cover;
      width: 100%;
    }

    .projects__meta {
      display: flex;
      flex-direction: column;
      gap: 0.2rem;
      min-width: 0;
    }

    .projects__name {
      color: var(--color-text-primary);
      font-size: 0.9375rem;
      font-weight: 600;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .projects__counts {
      color: var(--color-text-muted);
      display: flex;
      gap: 0.75rem;
      font-size: 0.6875rem;
    }

    .projects__count {
      align-items: center;
      display: inline-flex;
      gap: 0.25rem;
    }
  `,
})
export class ProjectsPageComponent {
  private readonly router = inject(Router);
  private readonly destroyRef = inject(DestroyRef);
  private readonly personalityService = inject(PersonalityService);
  private readonly chatService = inject(ChatService);
  private readonly fileAttachmentService = inject(FileAttachmentService);

  readonly loading = signal(true);
  readonly projects = signal<ProjectCard[]>([]);

  constructor() {
    forkJoin({
      personalities: this.personalityService.listPersonalities(1, 100).pipe(catchError(() => of(null))),
      docs: this.fileAttachmentService.listFileAttachments(1, 100).pipe(catchError(() => of(null))),
      chats: this.chatService.listChatsPage(1, 100).pipe(catchError(() => of(null))),
    })
      .pipe(takeUntilDestroyed(this.destroyRef), finalize(() => this.loading.set(false)))
      .subscribe(result => {
        const personalities = result.personalities?.results ?? [];
        const docCounts = new Map<string, number>();
        for (const doc of result.docs?.results ?? []) {
          if (doc.personality_id && !doc.file_type.startsWith('image/')) {
            docCounts.set(doc.personality_id, (docCounts.get(doc.personality_id) ?? 0) + 1);
          }
        }
        const threadCounts = new Map<string, number>();
        for (const chat of result.chats?.results ?? []) {
          if (chat.personality_id) {
            threadCounts.set(chat.personality_id, (threadCounts.get(chat.personality_id) ?? 0) + 1);
          }
        }
        this.projects.set(
          personalities.map(personality => ({
            personality,
            docCount: docCounts.get(personality.id) ?? 0,
            threadCount: threadCounts.get(personality.id) ?? 0,
          })),
        );
      });
  }

  openProject(card: ProjectCard): void {
    void this.router.navigate(['/projects', card.personality.id]);
  }
}
