import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  inject,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ActivatedRoute, Router } from '@angular/router';
import { AsyncPipe, DatePipe } from '@angular/common';
import { firstValueFrom, forkJoin, of } from 'rxjs';
import { catchError, finalize } from 'rxjs/operators';

import { Chat, DEFAULT_THREAD_NAME } from '../../core/models/chat.model';
import { Personality } from '../../core/models/personality.model';
import { FileAttachment } from '../../core/models/file-attachment.model';
import { PersonalityService } from '../../core/services/personality.service';
import { ChatService } from '../../core/services/chat.service';
import { FileAttachmentService } from '../../core/services/file-attachment.service';
import { AuthImagePipe } from '../../core/pipes/auth-image.pipe';
import {
  ArrowLeftIconComponent,
  ChatIconComponent,
  FileIconComponent,
  LayersIconComponent,
} from '../../shared/ui/icons/icons';

/**
 * Project workspace: one oracle's Construct documents and its chats side by
 * side. Upload/delete docs here, jump into any thread, or start a new one.
 */
@Component({
  selector: 'app-project-detail-page',
  imports: [
    AsyncPipe,
    DatePipe,
    AuthImagePipe,
    ArrowLeftIconComponent,
    ChatIconComponent,
    FileIconComponent,
    LayersIconComponent,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="pdetail">
      @if (loading()) {
        <p class="pdetail__status">Loading project…</p>
      } @else if (notFound()) {
        <div class="pdetail__notfound">
          <p class="pdetail__status">Project not found.</p>
          <button type="button" class="pdetail__btn" (click)="back()">Back to projects</button>
        </div>
      } @else if (personality(); as p) {
        <header class="pdetail__header">
          <button type="button" class="pdetail__back" (click)="back()" aria-label="Back to projects">
            <ui-arrow-left-icon [size]="16" />
          </button>
          <span class="pdetail__avatar">
            @if (p.cover_image_url; as cover) {
              <img [src]="cover | authImage | async" alt="" />
            } @else {
              <ui-layers-icon [size]="20" />
            }
          </span>
          <div class="pdetail__heading">
            <h1 class="pdetail__name">{{ p.name }}</h1>
            <p class="pdetail__sub">Project workspace: documents in the Construct, chats with this oracle.</p>
          </div>
          <button
            type="button"
            class="pdetail__btn pdetail__btn--accent"
            (click)="startNewChat()"
            [disabled]="startingChat()"
          >
            {{ startingChat() ? 'Starting…' : 'New chat' }}
          </button>
        </header>

        @if (error(); as err) {
          <p class="pdetail__error">{{ err }}</p>
        }

        <div class="pdetail__cols">
          <section class="pdetail__panel">
            <div class="pdetail__panel-head">
              <h2 class="pdetail__panel-title">Documents</h2>
              <button type="button" class="pdetail__btn" (click)="docInput.click()" [disabled]="uploading()">
                {{ uploading() ? 'Uploading…' : 'Upload' }}
              </button>
              <input
                #docInput
                type="file"
                class="pdetail__hidden"
                multiple
                accept=".pdf,.doc,.docx,.txt,.md,.csv,.json,.html,.css,.js,.ts,.py,.go,.java,.c,.cpp,.cs,.php,.xml,.rtf,.odt"
                (change)="onFilesSelected($event)"
              />
            </div>
            @if (docsLoading()) {
              <p class="pdetail__status">Loading documents…</p>
            } @else if (docs().length === 0) {
              <p class="pdetail__empty">No documents yet. Upload PDFs or text files; the oracle reads them in chat.</p>
            } @else {
              <ul class="pdetail__docs">
                @for (doc of docs(); track doc.id) {
                  <li class="pdetail__doc">
                    <ui-file-icon [size]="14" />
                    <span class="pdetail__doc-name">{{ doc.name }}</span>
                    @switch (doc.chunk_status) {
                      @case ('chunked') { <span class="pdetail__badge pdetail__badge--ok">indexed</span> }
                      @case ('failed') { <span class="pdetail__badge pdetail__badge--fail">failed</span> }
                      @case ('pending') { <span class="pdetail__badge pdetail__badge--pending">indexing…</span> }
                    }
                    <button
                      type="button"
                      class="pdetail__del"
                      (click)="deleteDoc(doc)"
                      [disabled]="deletingId() === doc.id"
                      [attr.aria-label]="'Delete ' + doc.name"
                    >×</button>
                  </li>
                }
              </ul>
            }
          </section>

          <section class="pdetail__panel">
            <div class="pdetail__panel-head">
              <h2 class="pdetail__panel-title">Chats</h2>
            </div>
            @if (threads().length === 0) {
              <p class="pdetail__empty">No chats yet. Start one and the oracle will read this project's documents.</p>
            } @else {
              <ul class="pdetail__threads">
                @for (thread of threads(); track thread.id) {
                  <li>
                    <button type="button" class="pdetail__thread" (click)="openThread(thread)">
                      <ui-chat-icon [size]="13" />
                      <span class="pdetail__thread-name">{{ thread.name }}</span>
                      <span class="pdetail__thread-date">{{ thread.created_at | date:'mediumDate' }}</span>
                    </button>
                  </li>
                }
              </ul>
            }
          </section>
        </div>
      }
    </div>
  `,
  styles: `
    .pdetail {
      display: flex;
      flex-direction: column;
      gap: 1.25rem;
      margin: 0 auto;
      max-width: 64rem;
      padding: 1.5rem 1rem 3rem;
    }

    .pdetail__header {
      align-items: center;
      display: flex;
      gap: 0.75rem;
    }

    .pdetail__back {
      align-items: center;
      background: transparent;
      border: 1px solid var(--color-border-base);
      border-radius: 0.5rem;
      color: var(--color-text-primary);
      cursor: pointer;
      display: flex;
      flex-shrink: 0;
      height: 2.25rem;
      justify-content: center;
      width: 2.25rem;
    }

    .pdetail__back:hover {
      background: var(--color-surface-elevated);
    }

    .pdetail__avatar {
      align-items: center;
      background: var(--color-surface-muted);
      border-radius: 0.625rem;
      display: flex;
      flex-shrink: 0;
      height: 3rem;
      justify-content: center;
      overflow: hidden;
      width: 3rem;
    }

    .pdetail__avatar img {
      height: 100%;
      object-fit: cover;
      width: 100%;
    }

    .pdetail__heading {
      display: flex;
      flex-direction: column;
      gap: 0.15rem;
      flex: 1;
      min-width: 0;
    }

    .pdetail__name {
      color: var(--color-text-primary);
      font-size: 1.25rem;
      font-weight: 600;
      margin: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .pdetail__sub {
      color: var(--color-text-muted);
      font-size: 0.75rem;
      margin: 0;
    }

    .pdetail__btn {
      background: var(--color-surface-elevated);
      border: 1px solid var(--color-border-base);
      border-radius: 0.5rem;
      color: var(--color-text-primary);
      cursor: pointer;
      flex-shrink: 0;
      font-size: 0.8125rem;
      padding: 0.5rem 0.875rem;
    }

    .pdetail__btn:hover {
      background: var(--color-surface-muted);
    }

    .pdetail__btn:disabled {
      cursor: wait;
      opacity: 0.6;
    }

    .pdetail__btn--accent {
      background: var(--color-accent);
      border-color: transparent;
      color: white;
      font-weight: 600;
    }

    .pdetail__error {
      background: color-mix(in srgb, #ef4444 12%, transparent);
      border-radius: 0.5rem;
      color: #f87171;
      font-size: 0.8125rem;
      margin: 0;
      padding: 0.5rem 0.75rem;
    }

    .pdetail__status {
      color: var(--color-text-muted);
      font-size: 0.875rem;
      padding: 2rem 0;
      text-align: center;
    }

    .pdetail__notfound {
      align-items: center;
      display: flex;
      flex-direction: column;
      gap: 0.75rem;
    }

    .pdetail__cols {
      display: grid;
      gap: 1rem;
      grid-template-columns: repeat(auto-fit, minmax(18rem, 1fr));
    }

    .pdetail__panel {
      background: var(--color-surface-base);
      border: 1px solid var(--color-border-base);
      border-radius: 0.75rem;
      display: flex;
      flex-direction: column;
      gap: 0.5rem;
      padding: 1rem;
    }

    .pdetail__panel-head {
      align-items: center;
      display: flex;
      gap: 0.75rem;
      justify-content: space-between;
    }

    .pdetail__panel-title {
      color: var(--color-text-primary);
      font-size: 0.9375rem;
      font-weight: 600;
      margin: 0;
    }

    .pdetail__hidden {
      display: none;
    }

    .pdetail__empty {
      color: var(--color-text-muted);
      font-size: 0.8125rem;
      margin: 0;
      padding: 1rem 0.25rem;
    }

    .pdetail__docs {
      display: flex;
      flex-direction: column;
      gap: 0.15rem;
      list-style: none;
      margin: 0;
      max-height: 22rem;
      overflow-y: auto;
      padding: 0;
    }

    .pdetail__doc {
      align-items: center;
      border-radius: 0.375rem;
      display: flex;
      gap: 0.5rem;
      padding: 0.45rem 0.5rem;
    }

    .pdetail__doc:hover {
      background: var(--color-surface-muted);
    }

    .pdetail__doc-name {
      color: var(--color-text-primary);
      flex: 1;
      font-size: 0.8125rem;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .pdetail__badge {
      border-radius: 999px;
      flex-shrink: 0;
      font-size: 0.625rem;
      padding: 0.1rem 0.45rem;
    }

    .pdetail__badge--ok {
      background: color-mix(in srgb, #22c55e 18%, transparent);
      color: #4ade80;
    }

    .pdetail__badge--fail {
      background: color-mix(in srgb, #ef4444 18%, transparent);
      color: #f87171;
    }

    .pdetail__badge--pending {
      background: color-mix(in srgb, #eab308 18%, transparent);
      color: #facc15;
    }

    .pdetail__del {
      background: transparent;
      border: none;
      border-radius: 0.375rem;
      color: var(--color-text-muted);
      cursor: pointer;
      flex-shrink: 0;
      font-size: 0.85rem;
      line-height: 1;
      padding: 0.15rem 0.3rem;
    }

    .pdetail__del:hover {
      color: #f87171;
    }

    .pdetail__del:disabled {
      cursor: wait;
      opacity: 0.5;
    }

    .pdetail__threads {
      display: flex;
      flex-direction: column;
      gap: 0.15rem;
      list-style: none;
      margin: 0;
      max-height: 22rem;
      overflow-y: auto;
      padding: 0;
    }

    .pdetail__thread {
      align-items: center;
      border-radius: 0.375rem;
      color: var(--color-text-primary);
      cursor: pointer;
      display: flex;
      gap: 0.5rem;
      padding: 0.45rem 0.5rem;
      text-align: left;
      width: 100%;
    }

    .pdetail__thread:hover {
      background: var(--color-surface-muted);
    }

    .pdetail__thread-name {
      flex: 1;
      font-size: 0.8125rem;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .pdetail__thread-date {
      color: var(--color-text-muted);
      flex-shrink: 0;
      font-size: 0.6875rem;
    }
  `,
})
export class ProjectDetailPageComponent {
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly destroyRef = inject(DestroyRef);
  private readonly personalityService = inject(PersonalityService);
  private readonly chatService = inject(ChatService);
  private readonly fileAttachmentService = inject(FileAttachmentService);

  readonly personality = signal<Personality | null>(null);
  readonly docs = signal<FileAttachment[]>([]);
  readonly threads = signal<Chat[]>([]);
  readonly loading = signal(true);
  readonly docsLoading = signal(false);
  readonly uploading = signal(false);
  readonly deletingId = signal<string | null>(null);
  readonly startingChat = signal(false);
  readonly error = signal<string | null>(null);
  readonly notFound = signal(false);

  constructor() {
    this.route.paramMap
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe(params => {
        const id = params.get('id');
        if (!id) {
          this.notFound.set(true);
          this.loading.set(false);
          return;
        }
        this.personalityService
          .getPersonality(id)
          .pipe(
            catchError(() => {
              this.notFound.set(true);
              return of(null);
            }),
            finalize(() => this.loading.set(false)),
            takeUntilDestroyed(this.destroyRef),
          )
          .subscribe(personality => {
            if (!personality) return;
            this.personality.set(personality);
            this.refreshDocs();
            this.refreshChats();
          });
      });
  }

  private refreshDocs(): void {
    const personality = this.personality();
    if (!personality) return;
    this.docsLoading.set(true);
    this.fileAttachmentService
      .listFileAttachments(1, 100, { personality_id: personality.id, docs_only: true })
      .pipe(
        takeUntilDestroyed(this.destroyRef),
        finalize(() => this.docsLoading.set(false)),
      )
      .subscribe({
        next: res => this.docs.set(res.results ?? []),
        error: () => this.error.set('Could not load the documents.'),
      });
  }

  private refreshChats(): void {
    const personality = this.personality();
    if (!personality) return;
    this.chatService
      .listChatsPage(1, 100, { personality_id: personality.id })
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: res => this.threads.set(res.results ?? []),
        error: () => this.error.set('Could not load the chats.'),
      });
  }

  onFilesSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    const files = Array.from(input.files ?? []);
    input.value = '';
    if (files.length === 0) return;
    const personality = this.personality();
    if (!personality) return;
    this.uploading.set(true);
    this.error.set(null);
    const uploads = files.map(file =>
      this.fileAttachmentService
        .uploadPersonalityFileAttachment(personality.id, file)
        .pipe(catchError(() => of(null))),
    );
    forkJoin(uploads)
      .pipe(
        finalize(() => {
          this.uploading.set(false);
          this.refreshDocs();
        }),
      )
      .subscribe(results => {
        const failed = results.filter(r => r === null).length;
        if (failed > 0) {
          this.error.set(
            failed === files.length
              ? 'Upload failed. Check the file type and size.'
              : 'Uploaded ' + (files.length - failed) + ' of ' + files.length + ' documents; the rest failed.',
          );
        }
      });
  }

  deleteDoc(doc: FileAttachment): void {
    this.deletingId.set(doc.id);
    this.error.set(null);
    this.fileAttachmentService
      .deleteFileAttachment(doc.id)
      .pipe(
        takeUntilDestroyed(this.destroyRef),
        finalize(() => this.deletingId.set(null)),
      )
      .subscribe({
        next: () => this.docs.set(this.docs().filter(d => d.id !== doc.id)),
        error: () => this.error.set('Could not delete the document.'),
      });
  }

  async startNewChat(): Promise<void> {
    const personality = this.personality();
    if (!personality || this.startingChat()) return;
    this.startingChat.set(true);
    this.error.set(null);
    try {
      const chat = await firstValueFrom(
        this.chatService.createChat({
          name: DEFAULT_THREAD_NAME,
          personality_id: personality.id,
        }),
      );
      this.chatService.setLastChatId(chat.id);
      void this.router.navigate(['/chat', chat.id]);
    } catch {
      this.error.set('Could not start a chat.');
      this.startingChat.set(false);
    }
  }

  openThread(chat: Chat): void {
    void this.router.navigate(['/chat', chat.id]);
  }

  back(): void {
    void this.router.navigate(['/projects']);
  }
}
