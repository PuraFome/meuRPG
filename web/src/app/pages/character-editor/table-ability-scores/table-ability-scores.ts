import { Component, computed, effect, inject, input, model, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { RouterLink } from '@angular/router';

import {
  canLower,
  canRaise,
  costOf,
  missingDie,
  pointsSpent,
  resultOfSet,
  resultOfValue,
  typedInRange,
} from '../../../core/characters/ability-methods';
import { describeCharacterError } from '../../../core/characters/character-errors';
import { abilityLabel } from '../../../core/characters/character-labels';
import { ABILITY_KEYS, type AbilityKey } from '../../../core/characters/characters.types';
import { emptyPlacement, freeCount, place, type Placement } from '../../../core/dice/dice';
import { SRD_521_LABEL } from '../../table-rules/method-cards/method-cards';
import { AbilityFields, type AbilityFormGroup } from '../ability-fields/ability-fields';
import { AbilityPlacing } from '../ability-scores/ability-placing/ability-placing';
import {
  CharacterEditorSource,
  type AbilityMethodKey,
  type AbilityRollsVm,
  type AbilityTableVm,
} from '../character-editor.types';

const WHEN = new Intl.DateTimeFormat('pt-BR', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' });

const WORDS: Readonly<Record<AbilityMethodKey, { tab: string; title: string; srd: boolean }>> = {
  standard_array: { tab: 'Padrão', title: 'Conjunto padrão', srd: true },
  point_buy: { tab: 'Pontos', title: 'Compra por pontos', srd: true },
  rolled_4d6: { tab: '4d6', title: '4d6, descartando o menor', srd: true },
  typed: { tab: 'Digitar', title: 'Digitar os valores', srd: false },
};

const ORDER: readonly AbilityMethodKey[] = ['standard_array', 'point_buy', 'rolled_4d6', 'typed'];

/**
 * The scores of the "Atributos" step when a **player** makes a new sheet (RN-24, E10-03 state 4): the ways the master
 * allows, as a radio group of up to four. Each writes the six numbers into the form, and the page sends the chosen way
 * with `CreateCharacter`, which the server checks again.
 *
 * - **Padrão:** the server's array, each value on one ability (the same placing as the 4d6).
 * - **Pontos:** a stepper per ability, "Restam N pontos" from the server's costs and budget (only additions).
 * - **4d6:** the six sets come from the server (`RollAbilityScores`, kept: the same ones on reload, never a roll in the
 *   browser). With physical dice the player types the dice once and the server stores them.
 * - **Digitar:** six fields, inside the server's range, before the race's bonus.
 *
 * Nothing here computes a modifier or a bonus: the sheet shows them after it is saved. Until the chosen way is complete the
 * step is `incomplete`, with the reason in `problem`, and the page refuses to save.
 */
@Component({
  selector: 'app-table-ability-scores',
  imports: [AbilityFields, AbilityPlacing, MatButtonModule, MatFormFieldModule, MatIconModule, MatInputModule, RouterLink],
  templateUrl: './table-ability-scores.html',
  styleUrl: './table-ability-scores.scss',
})
export class TableAbilityScores {
  private readonly source = inject(CharacterEditorSource);

  readonly campaignId = input.required<string>();
  readonly group = input.required<AbilityFormGroup>();
  readonly table = input.required<AbilityTableVm>();
  /** The chosen way, which the page sends with `CreateCharacter`. */
  readonly method = model<AbilityMethodKey>('typed');
  readonly incomplete = model(false);
  readonly problem = model('');

  protected readonly keys = ABILITY_KEYS;
  protected readonly abilityLabel = abilityLabel;
  protected readonly label = SRD_521_LABEL;

  /** The ways the master allows, in the order of the screen. */
  protected readonly allowed = computed(() =>
    ORDER.filter((k) =>
      k === 'standard_array' ? this.table().standardArray : k === 'point_buy' ? this.table().pointBuy : k === 'rolled_4d6' ? this.table().rolled4d6 : this.table().typed,
    ),
  );
  protected readonly words = WORDS;

  // The standard array.
  protected readonly arrayPlacement = signal<Placement>(emptyPlacement());
  protected readonly arrayResults = computed(() => this.table().standardValues.map(resultOfValue));

  // Point buy: every score starts at the lowest.
  protected readonly bought = signal<Record<AbilityKey, number>>({ str: 8, dex: 8, con: 8, int: 8, wis: 8, cha: 8 });
  protected readonly spent = computed(() => pointsSpent(this.bought(), this.table().pointBuyCosts, this.table().pointBuyMinScore));
  protected readonly left = computed(() => this.table().pointBuyBudget - this.spent());
  protected readonly costRows = computed(() => {
    const t = this.table();
    return t.pointBuyCosts.map((cost, i) => ({ score: t.pointBuyMinScore + i, cost }));
  });
  protected readonly cells = computed(() => Array.from({ length: this.table().pointBuyBudget }, (_, i) => i < this.spent()));

  // The 4d6.
  protected readonly rolls = signal<AbilityRollsVm | null>(null);
  protected readonly rollPlacement = signal<Placement>(emptyPlacement());
  protected readonly rolling = signal(false);
  protected readonly rollError = signal('');
  protected readonly rollResults = computed(() => (this.rolls()?.sets ?? []).map(resultOfSet));
  protected readonly rolledAt = computed(() => {
    const at = this.rolls()?.rolledAt;
    return at ? WHEN.format(at).replace(',', '') : '';
  });
  /** Physical dice: six rows of four, as text so an empty die reads as missing. */
  protected readonly dice = signal<string[][]>(Array.from({ length: 6 }, () => ['', '', '', '']));
  protected readonly diceMissing = computed(() => missingDie(this.dice()));

  constructor() {
    // The first way the master allows, and the roll the server already kept.
    effect(() => {
      const first = this.allowed()[0];
      if (first && !this.allowed().includes(this.method())) {
        this.method.set(first);
      }
    });
    let seeded = false;
    effect(() => {
      const t = this.table();
      if (!seeded) {
        seeded = true;
        this.method.set(this.allowed()[0] ?? 'typed');
        this.rolls.set(t.rolls);
        this.apply();
      }
    });
    // Typing in the fields of "Digitar" changes the form: the step is complete only inside the range.
    effect((onCleanup) => {
      const sub = this.group().valueChanges.subscribe(() => this.report());
      onCleanup(() => sub.unsubscribe());
    });
  }

  protected choose(method: AbilityMethodKey): void {
    this.method.set(method);
    this.apply();
  }

  /** Writes the active way's numbers into the form: a way not finished leaves 10 on the abilities still without one. */
  private apply(): void {
    const g = this.group();
    const set = (key: AbilityKey, value: number) => g.controls[key].setValue(value);
    switch (this.method()) {
      case 'standard_array':
        this.writePlacement(this.arrayPlacement(), this.arrayResults().map((r) => r.total), set);
        break;
      case 'rolled_4d6':
        this.writePlacement(this.rollPlacement(), this.rollResults().map((r) => r.total), set);
        break;
      case 'point_buy':
        ABILITY_KEYS.forEach((k) => set(k, this.bought()[k]));
        break;
      default: {
        const t = this.table();
        ABILITY_KEYS.forEach((k) => {
          const now = g.controls[k].value;
          set(k, typedInRange(now, t.typedMin, t.typedMax) ? now : Math.min(t.typedMax, Math.max(t.typedMin, 10)));
        });
      }
    }
    this.report();
  }

  private writePlacement(p: Placement, totals: readonly number[], set: (k: AbilityKey, v: number) => void): void {
    for (const key of ABILITY_KEYS) {
      const at = p[key];
      set(key, at === null ? 10 : totals[at]);
    }
  }

  /** What the page needs to know: whether this way is complete, and what is missing. */
  private report(): void {
    let problem = '';
    switch (this.method()) {
      case 'standard_array':
        problem = freeCount(this.arrayPlacement()) > 0 ? 'coloque cada valor do conjunto num atributo' : '';
        break;
      case 'rolled_4d6':
        problem =
          this.rolls() === null
            ? this.table().physicalDice ? 'digite os dados e guarde as rolagens' : 'role os atributos'
            : freeCount(this.rollPlacement()) > 0 ? 'coloque cada resultado num atributo' : '';
        break;
      case 'typed': {
        const t = this.table();
        const v = this.group().getRawValue();
        problem = ABILITY_KEYS.every((k) => typedInRange(v[k], t.typedMin, t.typedMax))
          ? ''
          : `digite valores de ${t.typedMin} a ${t.typedMax}`;
        break;
      }
      default:
        problem = '';
    }
    this.problem.set(problem);
    this.incomplete.set(problem !== '');
  }

  // The array and the 4d6 share the placing.
  protected placeArray(index: number, ability: AbilityKey): void {
    this.arrayPlacement.update((p) => place(p, index, ability));
    this.apply();
  }

  protected placeRoll(index: number, ability: AbilityKey): void {
    this.rollPlacement.update((p) => place(p, index, ability));
    this.apply();
  }

  // Point buy.
  protected costText(key: AbilityKey): number {
    return costOf(this.bought()[key], this.table().pointBuyCosts, this.table().pointBuyMinScore) ?? 0;
  }

  protected up(key: AbilityKey): boolean {
    const t = this.table();
    return canRaise(this.bought()[key], this.left(), t.pointBuyCosts, t.pointBuyMinScore);
  }

  protected down(key: AbilityKey): boolean {
    const t = this.table();
    return canLower(this.bought()[key], t.pointBuyCosts, t.pointBuyMinScore);
  }

  protected step(key: AbilityKey, delta: 1 | -1): void {
    if (delta === 1 ? !this.up(key) : !this.down(key)) {
      return;
    }
    this.bought.update((b) => ({ ...b, [key]: b[key] + delta }));
    this.apply();
  }

  // The 4d6.
  protected async rollInApp(): Promise<void> {
    await this.store(undefined);
  }

  protected async saveDice(): Promise<void> {
    if (this.diceMissing()) {
      return;
    }
    await this.store(this.dice().map((row) => row.map(Number)));
  }

  private async store(typed: readonly (readonly number[])[] | undefined): Promise<void> {
    if (this.rolling()) {
      return;
    }
    this.rolling.set(true);
    this.rollError.set('');
    try {
      const rolls = await this.source.rollAbilityScores(this.campaignId(), typed);
      this.rolls.set(rolls);
      this.rollPlacement.set(emptyPlacement());
      this.apply();
    } catch (err) {
      this.rollError.set(describeCharacterError(err));
    } finally {
      this.rolling.set(false);
    }
  }

  protected setDie(row: number, die: number, value: string): void {
    this.dice.update((all) => all.map((r, i) => (i === row ? r.map((d, j) => (j === die ? value : d)) : r)));
  }
}
