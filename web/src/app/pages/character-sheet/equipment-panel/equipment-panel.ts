import { Component, computed, input, output } from '@angular/core';

import { GearList, type GearToggle } from '../../../shared/gear-list/gear-list';
import { FullSheetVm } from '../character-sheet.types';
import { coinEntries, keepUnitsTogether } from '../sheet-format';

/**
 * "Equipamento": the gear the server lists, equipped and carried, with the button that moves a piece to the other group,
 * the armor class the server derives, the free-text items, the background's equipment as text and the coins. From 1200px
 * it sits in the fourth column, above "Características e traços".
 */
@Component({
  selector: 'app-equipment-panel',
  imports: [GearList],
  templateUrl: './equipment-panel.html',
  styleUrl: './equipment-panel.scss',
})
export class EquipmentPanel {
  readonly sheet = input.required<FullSheetVm>();
  /** Whether the gear list has its buttons: the owner while the sheet is theirs to change, the master always. */
  readonly canEquip = input(false);
  /** The piece whose change is being saved. */
  readonly equipBusyKey = input<string | null>(null);
  /** What the last change said ("Guardado: Espada longa. Classe de armadura 14."), for the live region under the list. */
  readonly equipNotice = input('');
  /** A refusal of the last change, shown beside the list. */
  readonly equipError = input('');
  /** A tap on "Equipar" / "Guardar": the page calls the server and reads the sheet again. */
  readonly gearToggled = output<GearToggle>();

  /** The background's equipment text, with each number kept beside its unit. */
  protected readonly backgroundEquipment = computed(() =>
    keepUnitsTogether(this.sheet().backgroundEquipment),
  );
  protected readonly coins = computed(() => coinEntries(this.sheet().coins));
}
