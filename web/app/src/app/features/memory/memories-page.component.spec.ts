import { NO_ERRORS_SCHEMA, provideZonelessChangeDetection } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, Router } from '@angular/router';
import { of } from 'rxjs';

import { MemoriesPageComponent } from './memories-page.component';
import { MemoriesListTabComponent } from './memories-list-tab.component';
import { MemoryMergeHistoryPageComponent } from './memory-merge-history-page.component';
import { CompactionLogPageComponent } from './compaction-log-page.component';
import { PersonalityService } from '../../core/services/personality.service';

describe('MemoriesPageComponent', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [MemoriesPageComponent],
      providers: [
        provideZonelessChangeDetection(),
        { provide: ActivatedRoute, useValue: { queryParams: of({}) } },
        { provide: Router, useValue: { navigate: vi.fn().mockName('navigate') } },
        { provide: PersonalityService, useValue: { listPersonalities: vi.fn().mockReturnValue(of({ results: [], total_count: 0, page: 1 })) } },
      ],
    })
      // Tabs load their own data; stub them out so this spec covers only the shell.
      .overrideComponent(MemoriesPageComponent, {
        remove: { imports: [MemoriesListTabComponent, MemoryMergeHistoryPageComponent, CompactionLogPageComponent] },
        add: { schemas: [NO_ERRORS_SCHEMA] },
      })
      .compileComponents();
  });

  it('creates with memories tab active by default', () => {
    const fixture = TestBed.createComponent(MemoriesPageComponent);
    expect(fixture.componentInstance).toBeTruthy();
    expect(fixture.componentInstance.activeTab()).toBe('memories');
  });

  it('renders a help hint next to the page title', () => {
    const fixture = TestBed.createComponent(MemoriesPageComponent);
    fixture.detectChanges();
    const trigger = (fixture.nativeElement as HTMLElement).querySelector('h1 ui-help-hint button');
    expect(trigger?.getAttribute('aria-label')).toBe('What is memory?');
  });
});
