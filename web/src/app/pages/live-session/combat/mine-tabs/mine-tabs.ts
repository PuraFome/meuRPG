import { Component, ElementRef, input, output, viewChildren } from '@angular/core';

import { type MineTab, tabWord } from '../../../../core/combat/mine';
import { mediaQuery } from '../../../../shared/map-view/media-query';

/**
 * The tabs of what a player plays (MR-037, E9-12 states 3 and 6): their character and its creatures, one tab each
 * ("Sálvia · Já agiu · 13", "Lobos atrozes (2) · Sua vez · 10"). A tablist of 48 px tabs in two columns; the
 * chosen one has an outline and a tint, and its word says where it stands in the order, so colour is never the
 * only sign. A tap picks what the page shows; the tabs never move. Arrow keys, Home and End move between them
 * (the chosen tab is the one in the tab order). It shows only when the player plays more than one combatant:
 * `CombatView` leaves it out otherwise. On a phone narrower than 360 px the word drops the initiative number.
 */
@Component({
  selector: 'app-mine-tabs',
  host: { '[attr.inert]': "inert() ? '' : null" },
  template: `
    <div class="tabs" role="tablist" aria-label="O que você joga">
      @for (t of tabs(); track t.id) {
        <button
          #tab
          type="button"
          role="tab"
          class="tab"
          [class.tab--on]="t.id === selected()"
          [class.tab--turn]="t.state === 'turn'"
          [attr.aria-selected]="t.id === selected()"
          [attr.tabindex]="t.id === selected() ? 0 : -1"
          [attr.data-tab]="t.id"
          (click)="select.emit(t.id)"
          (keydown)="key($event, $index)"
        >
          <span class="tab__name">{{ t.label }}</span>
          <span class="tab__word">{{ word(t) }}</span>
        </button>
      }
    </div>
  `,
  styles: `
    :host {
      display: block;
    }

    .tabs {
      display: grid;
      grid-auto-flow: column;
      grid-auto-columns: minmax(0, 1fr);
      gap: 8px;
    }

    .tab {
      display: flex;
      flex-direction: column;
      justify-content: center;
      align-items: flex-start;
      gap: 1px;
      box-sizing: border-box;
      min-width: 0;
      height: 48px;
      padding: 0 12px;
      border: 1px solid var(--mr-control-line);
      border-radius: var(--mr-radius-md);
      background: var(--mr-surface);
      color: var(--mr-ink);
      font: inherit;
      text-align: left;
      cursor: pointer;
    }

    // The chosen tab: an outline and a tint (never only the colour: its word and the page say it too).
    .tab--on {
      padding: 0 11px;
      border: 2px solid var(--mr-accent);
      background: var(--mr-accent-soft);
    }

    .tab__name {
      max-width: 100%;
      overflow: hidden;
      font-size: 15px;
      font-weight: 700;
      line-height: 18px;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .tab__word {
      max-width: 100%;
      overflow: hidden;
      font-size: 12.5px;
      line-height: 15px;
      color: var(--mr-ink-muted);
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .tab--turn .tab__word {
      color: var(--mr-accent-text);
      font-weight: 700;
    }
  `,
})
export class MineTabs {
  readonly tabs = input.required<readonly MineTab[]>();
  /** The id of the chosen tab. */
  readonly selected = input.required<string>();
  readonly select = output<string>();
  /** A question is open (ending a part): the tabs do nothing until it is answered. */
  readonly inert = input(false);

  private readonly buttons = viewChildren<ElementRef<HTMLButtonElement>>('tab');
  private readonly narrow = mediaQuery('(max-width: 359.98px)');

  protected word(t: MineTab): string {
    return tabWord(t, this.narrow());
  }

  /** The arrows, Home and End move the choice (and the focus) between the tabs. */
  protected key(event: KeyboardEvent, at: number): void {
    const n = this.tabs().length;
    const to =
      event.key === 'ArrowRight'
        ? (at + 1) % n
        : event.key === 'ArrowLeft'
          ? (at - 1 + n) % n
          : event.key === 'Home'
            ? 0
            : event.key === 'End'
              ? n - 1
              : -1;
    if (to < 0) {
      return;
    }
    event.preventDefault();
    this.select.emit(this.tabs()[to].id);
    queueMicrotask(() => this.buttons()[to]?.nativeElement.focus());
  }
}
