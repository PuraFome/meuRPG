import { Directive, input } from '@angular/core';

/** The panels the page projects into "Mais do combate" (the rail); "Registro" and "Efeitos" are the combat's own. */
export type MapTabId = 'fog' | 'traps' | 'treasure' | 'puzzles';

/**
 * Marks a panel the page hands to the combat to show in a row of its own: `<app-trap-panel appMapTab="traps"
 * [mapTabCount]="2">`. The row exists while the element does (a panel that does not apply is not drawn by the page, so its row
 * is not there).
 */
@Directive({ selector: '[appMapTab]' })
export class MapTab {
  readonly appMapTab = input.required<MapTabId>();
  /** The count in the row ("Armadilhas", 2); none when absent. */
  readonly mapTabCount = input<number | undefined>(undefined);
}
