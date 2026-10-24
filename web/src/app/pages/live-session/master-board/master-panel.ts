import { Directive, computed, input } from '@angular/core';

import { tabId, tabPanelId } from '../../../shared/tab-list/tab-list';
import type { MasterBoard } from './master-board';
import type { MasterTab } from './master-tabs';

/** The ids of the master board's tab buttons and panels start with this. */
export const BOARD_ID_PREFIX = 'mb';

/**
 * One panel of the master's board: named by its tab, always mounted and `hidden` while another tab is shown, so a draft in the
 * composer and the place in a list survive a change of tab. On a phone there is no tab list, so the panel is a plain region.
 */
@Directive({
  selector: '[appMasterPanel]',
  host: {
    '[attr.role]': "board().phone() ? 'region' : 'tabpanel'",
    '[id]': 'panelId()',
    '[attr.aria-labelledby]': 'board().phone() ? null : tabButtonId()',
    '[hidden]': '!shown()',
    class: 'mb__panel',
  },
})
export class MasterPanel {
  /** The tab this panel belongs to. */
  readonly appMasterPanel = input.required<MasterTab>();
  readonly board = input.required<MasterBoard>();

  protected readonly shown = computed(() => this.board().tab() === this.appMasterPanel());
  protected readonly panelId = computed(() => tabPanelId(BOARD_ID_PREFIX, this.appMasterPanel()));
  protected readonly tabButtonId = computed(() => tabId(BOARD_ID_PREFIX, this.appMasterPanel()));
}
