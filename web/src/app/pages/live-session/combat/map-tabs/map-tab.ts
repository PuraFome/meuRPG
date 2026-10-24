import { Directive, input } from '@angular/core';

/** The panels the page projects under the map; "Registro" and "Efeitos" are the combat's own. */
export type MapTabId = 'fog' | 'traps' | 'treasure' | 'puzzles';

/**
 * Marks a panel the page hands to the combat to show under the map, in a tab of its own: `<app-trap-panel appMapTab="traps"
 * [mapTabCount]="2">`. The tab exists while the element does (a panel that does not apply is not drawn by the page, so its tab
 * is not there), and the panel stays mounted, `hidden` when another tab is shown.
 */
@Directive({ selector: '[appMapTab]' })
export class MapTab {
  readonly appMapTab = input.required<MapTabId>();
  /** The count in the tab's name ("Armadilhas · 2"); none when absent. */
  readonly count = input<number | undefined>(undefined, { alias: 'mapTabCount' });
}
