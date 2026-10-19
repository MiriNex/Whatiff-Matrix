import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideZonelessChangeDetection } from '@angular/core';

import { ContextMusicTabComponent, withSolfeo, firstAbcBlock } from './context-music-tab.component';

describe('withSolfeo', () => {
  it('injects a w: solfège line after each music line', () => {
    const abc = ['X:1', 'T:t', 'M:3/4', 'L:1/4', 'K:C', 'C D E F | G A B c |]'].join('\n');
    const out = withSolfeo(abc);
    expect(out).toContain('w: Do Re Mi Fa * * * Sol La Si');
  });

  it('drops pre-existing lyric lines and re-derives them', () => {
    const abc = ['X:1', 'K:C', 'C D E', 'w: Do Re Mi viejo', 'F G A'].join('\n');
    const out = withSolfeo(abc);
    expect(out).not.toContain('viejo');
    expect(out).toContain('w: Fa Sol La');
  });

  it('marks accidentals in the syllable', () => {
    const abc = ['X:1', 'K:G', '^F _B'].join('\n');
    const out = withSolfeo(abc);
    expect(out).toContain('Fa#');
    expect(out).toContain('Sib');
  });
});

describe('firstAbcBlock', () => {
  it('extracts the first ```abc fenced block', () => {
    const text = 'blabla\n```abc\nX:1\nK:C\nC D E\n```\nmas texto';
    expect(firstAbcBlock(text)).toBe('X:1\nK:C\nC D E');
  });

  it('returns null when there is no abc block', () => {
    expect(firstAbcBlock('no music here')).toBeNull();
  });
});

describe('ContextMusicTabComponent', () => {
  let fixture: ComponentFixture<ContextMusicTabComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ContextMusicTabComponent],
      providers: [provideZonelessChangeDetection()],
    }).compileComponents();

    fixture = TestBed.createComponent(ContextMusicTabComponent);
    fixture.componentRef.setInput('chatId', 'chat-1');
    await fixture.whenStable();
  });

  it('seeds the editor with the welcome scale', () => {
    const el: HTMLElement = fixture.nativeElement;
    const ta = el.querySelector<HTMLTextAreaElement>('#music-abc-input');
    expect(ta?.value).toContain('T:Escala de Do en valsito');
  });

  it('renders the staff without errors', () => {
    const comp = fixture.componentInstance;
    comp.render();
    expect(comp.status()).toContain('partitura dibujada');
  });
});
