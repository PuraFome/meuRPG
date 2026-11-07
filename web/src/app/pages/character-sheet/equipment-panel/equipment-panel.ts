import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  effect,
  inject,
  input,
  output,
  untracked,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';

import { GearList, type GearToggle } from '../../../shared/gear-list/gear-list';
import { FullSheetVm, type WeaponMarkVm } from '../character-sheet.types';
import { WeaponMarks } from '../weapon-marks/weapon-marks';
import { coinEntries, keepUnitsTogether } from '../sheet-format';

/**
 * "Equipamento": the gear the server lists, equipped and carried, with the button that moves a piece to the other group,
 * the armor class the server derives, the free-text items, the background's equipment as text and the coins. From 1200px
 * it sits in the fourth column, above "Características e traços".
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-equipment-panel',
  imports: [GearList, MatButtonModule, WeaponMarks],
  templateUrl: './equipment-panel.html',
  styleUrl: './equipment-panel.scss',
})
export class EquipmentPanel {
  readonly sheet = input.required<FullSheetVm>();
  /** Whether the gear list has its buttons: for the owner and the master alike, unless the character is dead. The
   * sheet's lock does not matter: the gear is played. */
  readonly canEquip = input(false);
  /** The piece whose change is being saved. */
  readonly equipBusyKey = input<string | null>(null);
  /** What the last change said ("Guardado: Espada longa. Classe de armadura 14."), for the live region under the list. */
  readonly equipNotice = input('');
  /** A refusal of the last change, shown beside the list. */
  readonly equipError = input('');
  /** The sheet could not be read after a change: the error comes with "Tentar de novo". */
  readonly equipRetry = input(false);
  readonly equipRetried = output<void>();
  /** A tap on "Equipar" / "Guardar": the page calls the server and reads the sheet again. */
  readonly gearToggled = output<GearToggle>();

  /** Whether "Marcar as armas" is offered: the owner while the sheet is editable, the master always. */
  readonly canMark = input(false);
  /** The grid is open in place of its button. */
  readonly marking = input(false);
  readonly marksSaving = input(false);
  readonly marksError = input('');
  readonly marksNotice = input('');
  readonly marksOpened = output<void>();
  readonly marksSaved = output<WeaponMarkVm[]>();
  readonly marksClosed = output<void>();

  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);
  private wasMarking = false;

  /** The weapons of the sheet, equipped or carried: what the grid has a row for. */
  protected readonly weapons = computed(() => this.sheet().gear.filter((g) => g.kind === 'weapon'));

  constructor() {
    // The grid closed: the focus goes back to the button that opened it.
    effect(() => {
      const marking = this.marking();
      untracked(() => {
        if (this.wasMarking && !marking) {
          afterNextRender(
            () => this.host.nativeElement.querySelector<HTMLElement>('[data-marks-open]')?.focus(),
            { injector: this.injector },
          );
        }
        this.wasMarking = marking;
      });
    });
  }

  /** The background's equipment text, with each number kept beside its unit. */
  protected readonly backgroundEquipment = computed(() =>
    keepUnitsTogether(this.sheet().backgroundEquipment),
  );
  protected readonly coins = computed(() => coinEntries(this.sheet().coins));
}
