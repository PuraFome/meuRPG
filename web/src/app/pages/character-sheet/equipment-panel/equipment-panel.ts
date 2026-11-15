import { ChangeDetectionStrategy, Component, computed, inject, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { type AttunementEntryVm, MagicCategories } from '../../../core/characters/attunement';
import {
  type AmmunitionHint,
  type AmmunitionVm,
  type ChargesLine,
  type ItemChargesVm,
  ammunitionHint,
  chargesLine,
  recoverOf,
} from '../../../core/characters/charges';
import { type ConsumableNotice, magicRows } from '../../../core/characters/consumables';
import type { GearVm } from '../../../core/characters/gear';

import { reasonOf } from '../../../shared/attunement-section/attunement-rows';
import { ChipLegend } from '../../../shared/chip-legend/chip-legend';
import { CountPips } from '../../../shared/count-pips/count-pips';
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
  imports: [ChipLegend, CountPips, GearList, MatButtonModule, MatIconModule],
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

  /** Whether each row has its "⋮" for its marks or bonus: the owner while the sheet is editable, the master always. */
  readonly canMark = input(false);
  /** What the last save from a "⋮" said ("Marcas e bônus salvos.", "Bônus salvo."). */
  readonly marksNotice = input('');
  /** A tap on a row's "⋮": the page opens the sheet of its marks and bonus (a weapon) or its bonus (armor, shield). */
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

  /** What the last use of charges said, in the live region under the list: good (gastou) or bad (a refusal, a destroyed item). */
  readonly chargesResult = input<{ readonly text: string; readonly tone: 'good' | 'bad' } | null>(
    null,
  );
  /** A refusal of a charges call, in words. */
  readonly chargesError = input('');
  /** "Usar": the page opens the use sheet of the item. Works on a locked sheet. */
  readonly chargesUsed = output<AttunementEntryVm>();
  /** "Rolar": the page opens the d20 of the item's last charge. */
  readonly lastChargeRolled = output<AttunementEntryVm>();
  /** What the last use of a potion said, in the live region under the list. */
  readonly consumableResult = input<ConsumableNotice | null>(null);
  /** "Usar" on a potion or a scroll: the page opens the use sheet with one unit of the group. Works on a locked sheet. */
  readonly consumableUsed = output<AttunementEntryVm>();

  /** "Adicionar munição" (the owner while the sheet is open to editing, the master always). */
  readonly canAddAmmunition = input(false);
  /** The latest combat of the character is still running: "Recuperar a metade" waits for it to end. */
  readonly combatRunning = input(false);
  readonly ammunitionNotice = input('');
  readonly ammunitionError = input('');
  /** A tap on "Adicionar munição" (`null`) or on the hint's "Adicionar" (the kind and count the free text names). */
  readonly ammunitionAdding = output<AmmunitionHint | null>();
  readonly ammunitionRecovered = output<void>();

  protected readonly ammunition = computed((): readonly AmmunitionVm[] => this.sheet().ammunition);
  protected readonly recover = computed(() => recoverOf(this.ammunition()));
  protected readonly recoverOff = computed(
    () => this.combatRunning() || this.recover().state !== 'ready',
  );
  /** The free-text items look like ammunition while the app counts none. */
  protected readonly hint = computed(() =>
    this.ammunition().length === 0 ? ammunitionHint(this.sheet().equipment) : null,
  );
  /** The section shows with a list, with the hint, or for who can add the first kind. */
  protected readonly ammunitionShown = computed(
    () => this.ammunition().length > 0 || this.hint() !== null || this.canAddAmmunition(),
  );

  protected spentWords(a: AmmunitionVm): string {
    const fired = `${a.spent} ${a.spent === 1 ? 'gasta' : 'gastas'} no último combate`;
    if (a.back === 0) {
      return fired;
    }
    return a.recovered
      ? `${fired} · ${a.back} ${a.back === 1 ? 'voltou' : 'voltaram'}`
      : `${fired} · voltam ${a.back}`;
  }

  protected isLost(c: ItemChargesVm): boolean {
    return c.state === 'destroyed' || c.state === 'plain' || c.state === 'partly';
  }

  /** The dashed tag of a lost item: "Destruída", or "Sem magia" when it became a plain item or kept only its bonus. */
  protected tagOf(c: ItemChargesVm): string {
    return c.state === 'destroyed'
      ? 'Destruída'
      : c.state === 'plain' || c.state === 'partly'
        ? 'Sem magia'
        : '';
  }

  protected lineOf(e: AttunementEntryVm): ChargesLine | null {
    return e.charges ? chargesLine(e.charges, e.namePt) : null;
  }

  protected readonly magicItems = computed(() =>
    this.sheet().attunement.entries.filter((e) => e.kind === 'magic'),
  );
  /** The rows of the magic items: identical potions and scrolls are one row ("×2"), the rest as they were. */
  protected readonly rows = computed(() =>
    magicRows(this.magicItems(), (key) => this.categories.categoryOf(key)),
  );
  /** Some magic item holds charges: the lock line then names what still works on a locked sheet. */
  protected readonly hasCharges = computed(() => this.magicItems().some((e) => e.charges));
  /** Some row is a potion or a scroll, which "Usar" works for on a locked sheet too. */
  protected readonly hasConsumables = computed(() =>
    this.rows().some((r) => r.type === 'consumable'),
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

  /** The marks of the gear rows, for the legend. */
  protected readonly legendMarks = computed(() => this.sheet().gear.map((g) => g.marks));
  /** The bonus chip of the legend: solid when some bonus is added to the numbers now, dashed when none is, absent without one. */
  protected readonly legendBonus = computed((): '' | 'counts' | 'idle' => {
    const bonuses = this.sheet().gear.filter((g) => !!g.bonus);
    return bonuses.length === 0
      ? ''
      : bonuses.some((g) => g.bonusCounts !== false)
        ? 'counts'
        : 'idle';
  });

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
