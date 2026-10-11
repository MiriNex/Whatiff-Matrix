import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideZonelessChangeDetection } from '@angular/core';

import { BulkAction, BulkActionBarComponent } from './bulk-action-bar.component';

describe('BulkActionBarComponent', () => {
  let fixture: ComponentFixture<BulkActionBarComponent>;
  let chosen: string[];
  let cleared: number;

  const el = () => fixture.nativeElement as HTMLElement;
  const toggle = () => el().querySelector('.ui-bulk-bar__action-btn') as HTMLButtonElement;
  const options = () => Array.from(el().querySelectorAll('[role="option"]')) as HTMLButtonElement[];
  const actions: BulkAction[] = [
    { id: 'move', label: 'Move…', ariaLabel: 'Move 3 selected items' },
    { id: 'delete', label: 'Delete', danger: true },
  ];

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [BulkActionBarComponent],
      providers: [provideZonelessChangeDetection()],
    }).compileComponents();
    fixture = TestBed.createComponent(BulkActionBarComponent);
    fixture.componentRef.setInput('count', 3);
    fixture.componentRef.setInput('actions', actions);
    chosen = [];
    cleared = 0;
    fixture.componentInstance.action.subscribe(id => chosen.push(id));
    fixture.componentInstance.clear.subscribe(() => cleared++);
    fixture.detectChanges();
  });

  it('is a named region that says how many are selected', () => {
    const region = el().querySelector('[role="region"]');
    expect(region?.getAttribute('aria-label')).toBe('Bulk actions');
    expect(el().querySelector('[role="status"]')?.textContent?.trim()).toBe('3 selected');
    expect(toggle().getAttribute('aria-label')).toBe('Choose action for 3 selected');
  });

  it('opens the menu and runs an action on click (which Enter and Space also fire), then closes', () => {
    toggle().click();
    fixture.detectChanges();
    expect(toggle().getAttribute('aria-expanded')).toBe('true');
    expect(options().map(o => o.textContent?.trim())).toEqual(['Move…', 'Delete']);
    expect(options()[0].getAttribute('aria-label')).toBe('Move 3 selected items');
    expect(options()[1].hasAttribute('aria-label')).toBe(false);
    expect(options()[1].classList).toContain('ui-bulk-bar__menu-item--danger');

    options()[1].click();
    fixture.detectChanges();

    expect(chosen).toEqual(['delete']);
    expect(options()).toEqual([]);
  });

  it('keeps focus where it is when an option is pressed with the mouse', () => {
    toggle().click();
    fixture.detectChanges();
    const press = new MouseEvent('mousedown', { bubbles: true, cancelable: true });
    options()[0].dispatchEvent(press);
    expect(press.defaultPrevented).toBe(true);
  });

  it('closes on Escape and returns focus to the menu button', () => {
    toggle().click();
    fixture.detectChanges();
    el().querySelector('.ui-bulk-bar__action')?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    fixture.detectChanges();

    expect(fixture.componentInstance.menuOpen()).toBe(false);
    expect(document.activeElement).toBe(toggle());
  });

  it('stays open while focus moves into the menu, and closes when it leaves', () => {
    toggle().click();
    fixture.detectChanges();
    const root = el().querySelector('.ui-bulk-bar__action') as HTMLElement;

    root.dispatchEvent(new FocusEvent('focusout', { relatedTarget: options()[0] }));
    expect(fixture.componentInstance.menuOpen()).toBe(true);

    root.dispatchEvent(new FocusEvent('focusout', { relatedTarget: document.body }));
    expect(fixture.componentInstance.menuOpen()).toBe(false);
  });

  it('clears, and disables the menu and Clear with nothing selected', () => {
    (el().querySelector('.ui-bulk-bar__clear') as HTMLButtonElement).click();
    expect(cleared).toBe(1);

    fixture.componentRef.setInput('count', 0);
    fixture.detectChanges();
    expect(toggle().disabled).toBe(true);
    expect((el().querySelector('.ui-bulk-bar__clear') as HTMLButtonElement).disabled).toBe(true);
  });
});
