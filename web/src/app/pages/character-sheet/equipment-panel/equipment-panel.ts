import { ChangeDetectionStrategy, Component, computed, inject, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { type AttunementEntryVm, MagicCategories } from '../../../core/characters/attunement';
import type { GearVm } from '../../../core/characters/gear';

import { reasonOf } from '../../../shared/attunement-section/attunement-rows';
import { BonusChip } from '../../../shared/bonus-chip/bonus-chip';
import {
  type AttunementChip,
  GearList,
  type GearToggle,
} from '../../../shared/gear-list/gear-list';
import { FullSheetVm } from '../character-sheet.types';
import { coinEntries, keepUnitsTogether } from '../sheet-format';

/**
 * "Equipamento": the gear the server lists, equipped and carried, with the button that moves a piece to the other group,
 * the armor class the server derives, the free-text items, the background's equipment as text and the coins. From 1200px
 * it sits in the fourth column, above "Características e traços".
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-equipment-panel',
  imports: [BonusChip, GearList, MatButtonModule, MatIconModule],
  templateUrl: './equipment-panel.html',
  styleUrl: './equipment-panel.scss',
})
export class EquipmentPanel {
  readonly sheet = input.required<FullSheetVm>();
  /** Who looks: the master reads "Tor não cumpre", the owner "você não cumpre". */
  readonly viewer = input<'owner' | 'master'>('owner');
  /** The character's name, for what the master reads. */
  readonly characterName = input('');
  private readonly categories = inject(MagicCategories);
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

  /** Whether each weapon has its "⋮" for its marks: the owner while the sheet is editable, the master always. */
  readonly canMark = input(false);
  /** What the last save of marks said ("Marcas salvas."). */
  readonly marksNotice = input('');
  /** A tap on a weapon's "⋮": the page opens the sheet of its marks. */
  readonly marksOpened = output<GearVm>();

  /** The owner (while the sheet is editable) and the master add and remove magic items. */
  readonly canManageMagic = input(false);
  /** "Mudar" opens the sheet "Sintonização": the master always, the owner while a rest's window is open. */
  readonly canChangeAttunement = input(false);
  /** A dead character's owner: no "Mudar" and no "Muda num descanso". */
  readonly attunementIsClosed = input(false);
  /** What the last add or remove said. */
  readonly magicNotice = input('');
  readonly attunementOpened = output<void>();
  readonly magicAdding = output<void>();
  readonly magicRemoving = output<AttunementEntryVm>();

  protected readonly magicItems = computed(() =>
    this.sheet().attunement.entries.filter((e) => e.kind === 'magic'),
  );
  /** The header "Sintonizados: N de 3" shows when anything on the sheet asks for attunement or is attuned. */
  protected readonly hasAttunement = computed(() =>
    this.sheet().attunement.entries.some((e) => e.asks || e.attuned),
  );

  /** The attunement chip of each gear row, from the entry of the piece that names it. */
  protected readonly chipOf = computed(() => {
    const pieces = new Map(
      this.sheet()
        .attunement.entries.filter((e) => e.kind === 'piece' && e.gearKey)
        .map((e) => [e.gearKey, e] as const),
    );
    return (g: GearVm): AttunementChip => {
      const e = pieces.get(g.key);
      if (!e) {
        return '';
      }
      return e.attuned ? 'on' : e.asks ? 'asks' : '';
    };
  });

  /** Some gear row carries an attunement chip: the legend says what the two look like. */
  protected readonly showsChips = computed(() => {
    const chip = this.chipOf();
    return this.sheet().gear.some((g) => chip(g) !== '');
  });

  /** The marks that appear on some gear row, for the legend: the word and the icon of each chip. */
  protected readonly legendMarks = computed(() => {
    const gear = this.sheet().gear;
    return [
      { word: 'Prateada', icon: 'dark_mode', on: gear.some((g) => g.marks?.silvered) },
      { word: 'Adamantina', icon: 'diamond', on: gear.some((g) => g.marks?.adamantine) },
      { word: 'Mágica', icon: 'auto_awesome', on: gear.some((g) => g.marks?.magical) },
    ].filter((m) => m.on);
  });
  /** Some gear row carries a magic bonus chip. */
  protected readonly hasBonus = computed(() => this.sheet().gear.some((g) => !!g.bonus));
  /** Some bonus is added to the numbers now (the legend's chip is solid), else every one is dashed. */
  protected readonly bonusCounts = computed(() =>
    this.sheet().gear.some((g) => !!g.bonus && g.bonusCounts !== false),
  );
  /** The legend shows when any chip is on a row. */
  protected readonly hasLegend = computed(
    () => this.legendMarks().length > 0 || this.showsChips() || this.hasBonus(),
  );

  /** The line under a magic item that asks and is not attuned, with its icon; the same words as the short-rest sheet. */
  protected reasonOfEntry(e: AttunementEntryVm) {
    if (e.attuned || !e.asks) {
      return null;
    }
    const why = reasonOf(e, { viewer: this.viewer(), name: this.characterName() });
    return why.reason === '' ? null : why;
  }

  protected iconOf(e: AttunementEntryVm): string {
    return this.categories.iconOf(e.key);
  }

  protected pill(e: AttunementEntryVm): string {
    if (e.attuned) {
      return 'Sintonizado';
    }
    return e.asks ? 'Pede sintonização' : '';
  }

  /** The sheet lists a weapon: there is something to mark. */
  protected readonly hasWeapons = computed(() =>
    this.sheet().gear.some((g) => g.kind === 'weapon'),
  );

  /** The background's equipment text, with each number kept beside its unit. */
  protected readonly backgroundEquipment = computed(() =>
    keepUnitsTogether(this.sheet().backgroundEquipment),
  );
  protected readonly coins = computed(() => coinEntries(this.sheet().coins));
}
