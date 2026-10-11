import { ChangeDetectionStrategy, Component, ElementRef, input, output, signal, viewChild } from '@angular/core';

/** One entry in the bulk bar's "Choose action" menu. */
export interface BulkAction {
  id: string;
  label: string;
  /** Destructive (delete): shown in the danger color. */
  danger?: boolean;
  /** Accessible name when the visible label is not enough, e.g. one that says how many items it acts on. */
  ariaLabel?: string;
}

/**
 * The bar a selectable table shows above its rows: how many are selected, a "Choose action" menu
 * and Clear. Used by the Thread Manager and the gallery (grid select mode and list view).
 *
 * Content slots: `[uiBulkLead]` goes before the count (a select-all checkbox), the default slot
 * after it (a hint), and `[uiBulkTrail]` after Clear (a Done button).
 */
@Component({
  selector: 'ui-bulk-action-bar',
  standalone: true,
  templateUrl: './bulk-action-bar.component.html',
  styleUrl: './bulk-action-bar.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { '[class.ui-bulk-bar--card]': "variant() === 'card'" },
})
export class BulkActionBarComponent {
  readonly count = input.required<number>();
  readonly actions = input.required<readonly BulkAction[]>();
  /** Names the bar region and its menu. */
  readonly label = input('Bulk actions');
  /** `strip` spans a panel edge to edge (Thread Manager); `card` is a rounded box in a page (gallery). */
  readonly variant = input<'strip' | 'card'>('strip');

  /** The id of the chosen action. */
  readonly action = output<string>();
  readonly clear = output<void>();

  readonly menuOpen = signal(false);

  private readonly menuRoot = viewChild.required<ElementRef<HTMLElement>>('menuRoot');
  private readonly toggleButton = viewChild.required<ElementRef<HTMLButtonElement>>('toggleButton');

  toggleMenu(): void {
    this.menuOpen.update(open => !open);
  }

  run(action: BulkAction): void {
    this.menuOpen.set(false);
    this.action.emit(action.id);
  }

  /** Escape closes the menu and puts focus back on its button. */
  closeMenu(): void {
    if (!this.menuOpen()) return;
    this.menuOpen.set(false);
    this.toggleButton().nativeElement.focus();
  }

  /** Closes the menu once focus leaves it (tabbing from the button into the menu keeps it open). */
  onFocusOut(event: FocusEvent): void {
    const next = event.relatedTarget as Node | null;
    if (next && this.menuRoot().nativeElement.contains(next)) return;
    this.menuOpen.set(false);
  }
}
