import {
  ChangeDetectionStrategy,
  Component,
  Directive,
  computed,
  inject,
  input,
  model,
} from '@angular/core';

import { TabItem, TabList, tabId, tabPanelId } from '../tab-list/tab-list';

const RAIL_ID_PREFIX = 'rail';

/**
 * The right column of a live screen from 1024 px (the Conversa and, on the player's, the "Ficha"): an `aside` named by
 * its user, sticky under the bars, as tall as the screen, with its own scroll. Its tab list carries the counts in words,
 * and every panel (an `appRailPanel` element) stays mounted, `hidden` when inactive, so a draft in a composer and the place in a feed
 * survive a change of tab. Under 1024 px it is a plain block: the screens decide where it goes there (a phone has the pinned nav).
 * `--mr-rail-top` is the height of the bars above it (the app bar and the session bar, 120 px).
 */
@Component({
  selector: 'app-session-rail',
  imports: [TabList],
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { role: 'complementary', '[attr.aria-label]': 'label()' },
  template: `
    @if (tabs().length > 1) {
      <app-tab-list
        [label]="tabsLabel()"
        [tabs]="tabs()"
        [idPrefix]="prefix"
        [(selected)]="selected"
      />
    }
    <div class="rail__panels"><ng-content /></div>
  `,
  styleUrl: './session-rail.scss',
})
export class SessionRail {
  /** The aside's name, given by its user ("Conversa"). */
  readonly label = input.required<string>();
  /** What the tab list is called ("Seções da coluna"). */
  readonly tabsLabel = input('Seções da coluna');
  readonly tabs = input.required<readonly TabItem[]>();
  /** The tab shown: the first until one is chosen. */
  readonly selected = model('');

  protected readonly prefix = RAIL_ID_PREFIX;
  /** The shown tab's id. */
  readonly current = computed(() => {
    const tabs = this.tabs();
    return tabs.some((t) => t.id === this.selected()) ? this.selected() : (tabs[0]?.id ?? '');
  });
}

/** One panel of the rail, named by its tab; mounted always, `hidden` while another tab is shown. */
@Directive({
  selector: '[appRailPanel]',
  host: {
    role: 'tabpanel',
    '[id]': 'panelId()',
    '[attr.aria-labelledby]': 'tabButtonId()',
    '[hidden]': '!shown()',
    class: 'rail__panel',
  },
})
export class RailPanel {
  private readonly rail = inject(SessionRail);
  /** The id of the tab this panel belongs to. */
  readonly appRailPanel = input.required<string>();

  protected readonly shown = computed(() => this.rail.current() === this.appRailPanel());
  protected readonly panelId = computed(() => tabPanelId(RAIL_ID_PREFIX, this.appRailPanel()));
  protected readonly tabButtonId = computed(() =>
    this.rail.tabs().length > 1 ? tabId(RAIL_ID_PREFIX, this.appRailPanel()) : null,
  );
}
