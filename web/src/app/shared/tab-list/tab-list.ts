import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  computed,
  input,
  model,
  viewChildren,
} from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/** One tab of a `TabList`. */
export interface TabItem {
  readonly id: string;
  readonly label: string;
  readonly icon?: string;
  /** The count as the eye reads it ("3", "99+"); empty or absent draws none. */
  readonly badge?: string;
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
    <div class="tl" role="tablist" [attr.aria-label]="label()">
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
            <span class="tl__badge" aria-hidden="true">{{ t.badge }}</span>
          }
        </button>
      }
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

  private readonly buttons = viewChildren<ElementRef<HTMLButtonElement>>('tab');

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
