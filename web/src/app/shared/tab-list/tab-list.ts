import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  afterRenderEffect,
  DestroyRef,
  computed,
  inject,
  input,
  model,
  signal,
  viewChild,
  viewChildren,
} from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/** What a count says (C6-10): `wait`, amber, something waits for the person; `news`, grená, new messages; `count`, neutral, how many there are. */
export type TabTone = 'wait' | 'news' | 'count';

/** The edge fade of a list that scrolls sideways, and how near the edge the fade starts. */
const EDGE_SLACK = 1;

/** One tab of a `TabList`. */
export interface TabItem {
  readonly id: string;
  readonly label: string;
  readonly icon?: string;
  /** The count as the eye reads it ("3", "99+"); empty or absent draws none. */
  readonly badge?: string;
  /** What the count means; neutral when absent. Colour is never alone: `words` says the same. */
  readonly tone?: TabTone;
  /** The count in words, appended to the tab's name ("3 mensagens novas"): the name is "Conversa, 3 mensagens novas". */
  readonly words?: string;
}

/** The id of a tab's button, and of its panel, for a list named `prefix`: the panels wire `aria-labelledby` and `aria-controls` with these. */
export const tabId = (prefix: string, id: string): string => `${prefix}-tab-${id}`;
export const tabPanelId = (prefix: string, id: string): string => `${prefix}-panel-${id}`;

/**
 * The ARIA tabs pattern, drawn as an underlined row: one tab stop (the selected tab), the arrow keys, Home and End move
 * and select, and each tab's name carries its count in words ("Conversa, 3 mensagens novas"). The panels are the
 * caller's: they keep their ids from `tabPanelId` and stay mounted, `hidden` when inactive. A row of route links is a
 * `nav`, not this; a radio choice is `app-segmented`.
 */
@Component({
  selector: 'app-tab-list',
  imports: [MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="tl-wrap" [class.tl-wrap--start]="fadeStart()" [class.tl-wrap--end]="fadeEnd()">
    <div #list class="tl" role="tablist" [class.tl--compact]="compact()" [attr.aria-label]="label()" (scroll)="measure()">
      @for (t of tabs(); track t.id) {
        <button
          #tab
          type="button"
          role="tab"
          class="tl__tab"
          [id]="idOf(t.id)"
          [class.tl__tab--on]="t.id === current()"
          [attr.aria-selected]="t.id === current()"
          [attr.aria-controls]="panelOf(t.id)"
          [attr.aria-label]="nameOf(t)"
          [attr.tabindex]="t.id === current() ? 0 : -1"
          (click)="select(t.id)"
          (keydown)="onKey($event)"
        >
          @if (t.icon) {
            <mat-icon aria-hidden="true">{{ t.icon }}</mat-icon>
          }
          <span>{{ t.label }}</span>
          @if (t.badge) {
            <span class="tl__badge" [class]="'tl__badge tl__badge--' + (t.tone ?? 'count')" aria-hidden="true">{{ t.badge }}</span>
          }
        </button>
      }
    </div>
    </div>
  `,
  styleUrl: './tab-list.scss',
})
export class TabList {
  /** What the list is called to a screen reader ("Seções da conversa"). */
  readonly label = input.required<string>();
  readonly tabs = input.required<readonly TabItem[]>();
  /** The ids of the tab buttons and of their panels start with this. */
  readonly idPrefix = input.required<string>();
  /** The selected tab's id; the first tab until one is chosen. */
  readonly selected = model('');
  /** The narrow look of a list that does not fit a phone: smaller type and padding, so more tabs show. */
  readonly compact = input(false);

  /** More tabs lie beyond the left or the right edge: the fade says so. */
  protected readonly fadeStart = signal(false);
  protected readonly fadeEnd = signal(false);
  private readonly list = viewChild<ElementRef<HTMLElement>>('list');

  private observer?: ResizeObserver;
  private readonly buttons = viewChildren<ElementRef<HTMLButtonElement>>('tab');

  constructor() {
    // The list scrolls sideways when it is wider than its room: the open tab stays in view and the edges fade.
    afterRenderEffect(() => {
      this.tabs();
      this.current();
      this.reveal();
      this.measure();
      this.watch();
    });
    inject(DestroyRef).onDestroy(() => this.observer?.disconnect());
  }

  /** The selected tab, or the first when `selected` names none of them. */
  protected readonly current = computed(() => {
    const tabs = this.tabs();
    return tabs.some((t) => t.id === this.selected()) ? this.selected() : (tabs[0]?.id ?? '');
  });

  protected idOf(id: string): string {
    return tabId(this.idPrefix(), id);
  }

  protected panelOf(id: string): string {
    return tabPanelId(this.idPrefix(), id);
  }

  protected measure(): void {
    const el = this.list()?.nativeElement;
    if (!el) {
      return;
    }
    this.fadeStart.set(el.scrollLeft > EDGE_SLACK);
    this.fadeEnd.set(el.scrollLeft + el.clientWidth < el.scrollWidth - EDGE_SLACK);
  }

  /**
   * Keeps the selected tab in view, and the list cut only between tabs: when the tab is not whole in the room, the list
   * scrolls to the start of the tab before it (so the person sees where they came from), never to the middle of a tab,
   * which would leave a bare count at the edge.
   */
  private reveal(): void {
    const el = this.list()?.nativeElement;
    const buttons = this.buttons().map((b) => b.nativeElement);
    const at = buttons.findIndex((b) => b.id === this.idOf(this.current()));
    if (!el || at < 0) {
      return;
    }
    const tab = buttons[at];
    if (
      tab.offsetLeft >= el.scrollLeft &&
      tab.offsetLeft + tab.offsetWidth <= el.scrollLeft + el.clientWidth
    ) {
      return;
    }
    const before = buttons[at - 1] ?? tab;
    el.scrollLeft = before.offsetLeft;
    if (tab.offsetLeft + tab.offsetWidth > el.scrollLeft + el.clientWidth) {
      el.scrollLeft = tab.offsetLeft + tab.offsetWidth - el.clientWidth;
    }
  }

  /** Fonts and the room change after the first render: the fade and the open tab follow. */
  private watch(): void {
    this.observer?.disconnect();
    const el = this.list()?.nativeElement;
    if (!el || typeof ResizeObserver === 'undefined') {
      return;
    }
    this.observer = new ResizeObserver(() => {
      this.reveal();
      this.measure();
    });
    this.observer.observe(el);
    this.buttons().forEach((b) => this.observer?.observe(b.nativeElement));
  }

  protected nameOf(tab: TabItem): string | null {
    return tab.words ? `${tab.label}, ${tab.words}` : null;
  }

  protected select(id: string): void {
    this.selected.set(id);
  }

  protected onKey(event: KeyboardEvent): void {
    const count = this.tabs().length;
    const at = this.tabs().findIndex((t) => t.id === this.current());
    let to: number;
    switch (event.key) {
      case 'ArrowRight':
        to = (at + 1) % count;
        break;
      case 'ArrowLeft':
        to = (at - 1 + count) % count;
        break;
      case 'Home':
        to = 0;
        break;
      case 'End':
        to = count - 1;
        break;
      default:
        return;
    }
    event.preventDefault();
    this.select(this.tabs()[to].id);
    this.buttons()[to]?.nativeElement.focus();
  }
}
