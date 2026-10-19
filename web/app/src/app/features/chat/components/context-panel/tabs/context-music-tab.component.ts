import { ChangeDetectionStrategy, Component, ElementRef, effect, inject, input, signal, viewChild } from '@angular/core';

import { ButtonComponent } from '../../../../../shared/ui/button/button.component';
import { HelpHintComponent } from '../../../../../shared/ui/help-hint/help-hint.component';

declare const ABCJS: {
  renderAbc(target: string | HTMLElement, code: string, options?: Record<string, unknown>): unknown[];
  synth: {
    supportsAudio(): boolean;
    CreateSynth: new () => {
      init(opts: { visualObj: unknown; options?: Record<string, unknown> }): Promise<void>;
      prime(): Promise<void>;
      start(): void;
      stop(): void;
    };
  };
};

const SOLFEO: Record<string, string> = { C: 'Do', D: 'Re', E: 'Mi', F: 'Fa', G: 'Sol', A: 'La', B: 'Si' };

const WELCOME_ABC = `X:1
T:Escala de Do en valsito
C:Nex para Miriam
M:3/4
L:1/4
Q:1/4=90
K:C
D E F | G A B | c3 | B A G | F E D | C3 |]`;

function tokenizeLine(line: string): string[] {
  const tokens: string[] = [];
  const re = /(\^|_)?([A-Ga-g])([,']*)(\d*\/\d+|\d+)?|(z|Z)(\d+)?|\|\]|\|/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(line)) !== null) {
    if (m[2]) {
      let syl = SOLFEO[m[2].toUpperCase()] ?? '';
      if (m[1] === '^') syl += '#';
      if (m[1] === '_') syl += 'b';
      tokens.push(syl);
    } else if (m[5]) {
      tokens.push('z');
    } else if (m[0] === '|]') {
      tokens.push('|]');
    } else if (m[0] === '|') {
      tokens.push('|');
    }
  }
  return tokens;
}

function tokensToLyricLine(tokens: string[]): string {
  const out: string[] = [];
  for (const t of tokens) {
    if (t === '|' || t === '|]') {
      out.push('*', '*', '*');
    } else {
      out.push(t);
    }
  }
  return 'w: ' + out.join(' ');
}

export function withSolfeo(abc: string): string {
  const lines = abc.split('\n');
  const out: string[] = [];
  let inMusic = false;
  for (const line of lines) {
    if (line.startsWith('w:')) continue;
    out.push(line);
    if (line.startsWith('K:')) {
      inMusic = true;
      continue;
    }
    const isHeader = /^[A-Za-z]:/.test(line);
    if (inMusic && !isHeader && line.trim() !== '' && /[A-Ga-gz]/.test(line)) {
      out.push(tokensToLyricLine(tokenizeLine(line)));
    }
  }
  return out.join('\n');
}

export function firstAbcBlock(text: string): string | null {
  const m = /```abc\s*\n([\s\S]*?)```/.exec(text);
  return m ? m[1].trimEnd() : null;
}

@Component({
  selector: 'app-context-music-tab',
  standalone: true,
  imports: [ButtonComponent, HelpHintComponent],
  template: `
    <section class="tab-body" aria-label="Music room">
      <div class="label-row">
        <label for="music-abc-input" class="label">Sala de partituras</label>
        <ui-help-hint label="How does this work?" heading="ABC notation" guide="continuity" align="end">
          Paste ABC notation (or a \`\`\`abc block from the chat) and it renders as a real staff
          with Do Re Mi syllables under each note. Hit play to hear it.
        </ui-help-hint>
      </div>
      <textarea
        id="music-abc-input"
        [value]="abcSource()"
        (input)="abcSource.set($any($event.target).value)"
        placeholder="X:1&#10;T:Mi ejercicio&#10;M:3/4&#10;L:1/4&#10;K:C&#10;C D E F | G A B c |]"
        aria-label="ABC notation source"
      ></textarea>
      <div class="actions">
        <ui-button size="sm" variant="primary" (activate)="render()">âœ¦ Draw</ui-button>
        <ui-button size="sm" variant="secondary" (activate)="play()">â–¶ Play</ui-button>
        <ui-button size="sm" variant="secondary" (activate)="stop()">â–  Stop</ui-button>
      </div>
      <label class="chk">
        <input type="checkbox" [checked]="solfeoOn()" (change)="toggleSolfeo()" />
        Do Re Mi bajo las notas
      </label>
      @if (status(); as s) {
        <p [class.error]="statusKind() === 'error'" class="status">{{ s }}</p>
      }
      <div #paper class="paper"></div>
    </section>
  `,
  styles: [
    `
      .tab-body { display: grid; gap: 0.6rem; }
      .label-row { align-items: center; display: flex; justify-content: space-between; gap: 0.5rem; }
      .label { color: var(--color-text-muted); font-size: 0.75rem; font-weight: 700; text-transform: uppercase; }
      textarea {
        background: var(--color-surface-base);
        border: 1px solid var(--color-border-base);
        border-radius: 0.75rem;
        color: var(--color-text-primary);
        font-family: 'Cascadia Code', Consolas, monospace;
        font-size: 0.8rem;
        min-height: 8rem;
        padding: 0.65rem;
        resize: vertical;
        width: 100%;
      }
      .actions { display: flex; flex-wrap: wrap; gap: 0.5rem; }
      .chk { align-items: center; color: var(--color-text-muted); cursor: pointer; display: flex; font-size: 0.8rem; gap: 0.4rem; }
      .status { color: var(--color-text-muted); font-size: 0.8rem; margin: 0; }
      .status.error { color: var(--color-danger); }
      .paper { background: #fbf8f2; border-radius: 0.75rem; min-height: 12rem; overflow-x: auto; padding: 0.9rem; }
    `,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ContextMusicTabComponent {
  readonly chatId = input<string | null>(null);

  readonly abcSource = signal<string>('');
  readonly solfeoOn = signal<boolean>(true);
  readonly status = signal<string | null>(null);
  readonly statusKind = signal<'ok' | 'error'>('ok');

  private readonly paperEl = viewChild.required<ElementRef<HTMLDivElement>>('paper');
  private rendered: unknown[] | null = null;
  private synth: { stop(): void } | null = null;
  private seeded = false;

  constructor() {
    effect(() => {
      if (this.chatId() && !this.seeded) {
        this.seeded = true;
        this.abcSource.set(WELCOME_ABC);
        queueMicrotask(() => this.render());
      }
    });
  }

  render(): void {
    const target = this.paperEl().nativeElement;
    target.innerHTML = '';
    this.rendered = null;
    const source = this.solfeoOn() ? withSolfeo(this.abcSource()) : this.abcSource();
    if (!source.trim()) {
      this.status.set('Escribe o pega ABC â€” o pide un ejercicio ```abc a tu personalidad.');
      this.statusKind.set('ok');
      return;
    }
    try {
      this.rendered = ABCJS.renderAbc(target, source, {
        paddingbottom: 14,
        paddingleft: 14,
        paddingright: 14,
        paddingtop: 10,
        responsive: 'resize',
      });
      this.status.set('âœ¦ partitura dibujada â€” dale a Play para oÃ­rla');
      this.statusKind.set('ok');
    } catch (e) {
      this.status.set('ABC no cuadra: ' + (e instanceof Error ? e.message : String(e)));
      this.statusKind.set('error');
    }
  }

  async play(): Promise<void> {
    if (!this.rendered || !this.rendered.length) {
      this.render();
      if (!this.rendered || !this.rendered.length) return;
    }
    if (!ABCJS.synth.supportsAudio()) {
      this.status.set('Tu navegador no soporta el sintetizador â€” prueba Chrome o Edge.');
      this.statusKind.set('error');
      return;
    }
    this.stop();
    try {
      const synth = new ABCJS.synth.CreateSynth();
      await synth.init({ visualObj: this.rendered[0], options: {} });
      await synth.prime();
      synth.start();
      this.synth = synth;
      this.status.set('â™ª sonando â€” escucha, mi reina');
      this.statusKind.set('ok');
    } catch (e) {
      this.status.set('Audio: ' + (e instanceof Error ? e.message : String(e)));
      this.statusKind.set('error');
    }
  }

  stop(): void {
    if (this.synth) {
      try {
        this.synth.stop();
      } catch {
        // already stopped
      }
      this.synth = null;
    }
  }

  toggleSolfeo(): void {
    this.solfeoOn.set(!this.solfeoOn());
    this.render();
  }
}

