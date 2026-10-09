import { Component, ElementRef, computed, effect, inject, signal, viewChild } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  AttackOutcome,
  type AttackRoll,
  CombatantSide,
  type PendingDamage,
  type TargetInReach,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import type { Attack, BonusAttackRule } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { effectivePreference } from '../../../../core/campaigns/dice-labels';
import {
  type AttackStage,
  awaitsReaction,
  isHit,
  outcomeWord,
  stageAfterRoll,
  steps,
  targetAfter,
  targetRows,
} from '../../../../core/combat/attack-flow';
import {
  type AttackDie,
  type DamageDie,
  CombatClient,
} from '../../../../core/combat/combat-client';
import { ActionKey } from '../../../../core/connect/idempotency';
import {
  damageFormula,
  diceName,
  extraDiceOf,
  rollFormula,
  sumRange,
} from '../../../../core/combat/combat-dice';
import {
  brutalLabel,
  brutalTyped,
  brutalTypedHint,
  criticalHint,
  criticalSentence,
  criticalSum,
  criticalTypedHint,
  fixedParts,
  hasExtraDice,
  typedRange,
} from '../../../../core/combat/critical';
import { reactionWait } from '../../../../core/combat/reactions';
import { isTheatre } from '../../../../core/combat/theatre';
import { metersText } from '../../../../core/units';
import { combatErrorMessage } from '../../../../core/combat/combat-errors';
import type { CombatState } from '../../../../core/combat/combat-state';
import {
  attackDetail,
  attackName,
  beamSpentText,
  bonusSpentText,
  isCantrip,
} from '../../../../core/combat/combat-options';
import { article } from '../../../../core/combat/combat-log';
import { joinDots, tight } from '../../../../core/format/text';
import { combatantInitial, isPlayer, stateWord } from '../../../../core/combat/combat-view';
import { isCreature } from '../../../../core/combat/creature-names';
import { CombatantToken } from '../../../../shared/combatant-token/combatant-token';
import { AttackResult } from './attack-result';
import { AttackSteps } from './attack-steps';
import { RollPicker } from '../roll-picker/roll-picker';
import { injectSheet } from '../sheet-host';

/** What the page hands the attack sheet. */
export interface AttackSheetData {
  readonly campaignId: string;
  readonly encounterId: string;
  readonly attackerId: string;
  readonly round: number;
  readonly attack: Attack;
  /** The attack's targets from `GetTurnOptions`, in turn order. */
  readonly targets: readonly TargetInReach[];
  readonly diceMode: DiceMode;
  readonly preference: DicePreference;
  /** Where each answer's combat goes (the page's copy of the combat). */
  readonly state: CombatState;
  /** An opportunity attack: a melee attack off turn that spends the reaction
   * instead of the action (`RollAttack.as_reaction`). */
  readonly asReaction?: boolean;
  /** Extra Attack: how many attacks of the Attack action remain before this
   * one, and how many it makes. */
  readonly attacksLeft?: number;
  readonly attacksPerAction?: number;
  /** A bonus action attack (off hand, Artes Marciais, Rajada de Golpes): it
   * spends the bonus action, not the action; a Rajada de Golpes strike counts
   * down the `bonusAttacksLeft` the options had. */
  readonly bonusRule?: BonusAttackRule;
  readonly bonusAttacksLeft?: number;
  /** A cantrip with several beams (Eldritch Blast): how many were still to fire before this one. */
  readonly beamsLeft?: number;
  /** The answer to an opportunity offer (E9-13): the target is the mover (so the
   * sheet starts at "Rolar"), the reach is not checked (the attack comes right
   * before it leaves), and the roll names the offer. `byMaster` when the master
   * rolls it for an NPC. */
  readonly opportunity?: {
    readonly offerId: string;
    readonly targetId: string;
    readonly targetLabel: string;
    readonly byMaster: boolean;
  };
  /** A hit whose damage was never rolled (the sheet was closed): the sheet
   * opens at "Dano" with it. */
  readonly resume?: { readonly pending: PendingDamage; readonly targetLabel: string };
  /** The monk's throw back after Defletir Projéteis: the reaction window that caught the missile
   * (`RollAttack.catch_window_id`). It is part of the same reaction, so it asks nothing more. */
  readonly catchWindowId?: string;
}

/**
 * "Atacar com Raio de Fogo" (MR-012, MR-014, E6-07, E6-08): the player's
 * attack in three steps, Alvo, Rolar and Dano, in a bottom sheet on a phone
 * and a dialog from a tablet up. It calls `RollAttack` and `RollDamage`
 * itself and hands each answer's combat to the page's `CombatState`. A
 * player never sees an armor class: the result says only "Acertou",
 * "Crítico" or "Errou". The two keys (one for the d20, one for the damage)
 * are made once, so a tap repeated after a lost answer never rolls twice.
 * The damage of a hit that closed before its roll comes back as `resume`.
 * Focus: the title opens first; after the result it goes to "Voltar à sua vez".
 */
@Component({
  selector: 'app-attack-sheet',
  imports: [AttackResult, AttackSteps, CombatantToken, MatButtonModule, MatIconModule, RollPicker],
  templateUrl: './attack-sheet.html',
  styleUrl: './attack-sheet.scss',
})
export class AttackSheet {
  private readonly api = inject(CombatClient);
  private readonly sheet = injectSheet<AttackSheetData, boolean>();
  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;
  protected readonly attack = this.data.attack;

  protected readonly stage = signal<AttackStage>(
    this.data.resume ? 'damage' : this.data.opportunity ? 'roll' : 'target',
  );
  protected readonly targetId = signal<string | null>(this.data.opportunity?.targetId ?? null);
  protected readonly typing = signal(false);
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  protected readonly roll = signal<AttackRoll | null>(null);
  protected readonly pending = signal<PendingDamage | null>(this.data.resume?.pending ?? null);
  /** The damage rolled, once it is. */
  protected readonly damage = signal<PendingDamage | null>(null);

  /** One key per attack roll (target and die): the same values again are a retry, others a new attack. */
  private readonly attackKeys = new ActionKey();
  /** One key per damage roll: the same die again is a retry, another die (typed after an app roll was lost) is a new request. */
  private readonly damageKeys = new ActionKey();
  private readonly body = viewChild<ElementRef<HTMLElement>>('body');
  private readonly back = viewChild('back', { read: ElementRef<HTMLButtonElement> });

  protected readonly name = attackName(this.attack);
  // An opportunity attack is a melee attack: a thrown dagger reads "corpo a
  // corpo" here, not its thrown range.
  protected readonly detail = this.data.asReaction
    ? `Reação · ${attackDetail({ ...this.attack, rangeFt: 5, longRangeFt: 0 })}`
    : `Ação · ${attackDetail(this.attack)}`;
  protected readonly cantrip = isCantrip(this.attack);
  protected readonly rangeText = metersText(this.data.asReaction ? 5 : this.attack.rangeFt);
  /** An opportunity attack reaches 5 ft, whatever range the weapon has when thrown. */
  protected readonly rows = computed(() => this.withAllies(this.baseRows()));
  /** An ally is tagged ("Aliada"), so a table of friends does not misread the list. */
  private withAllies(rows: ReturnType<typeof targetRows>): ReturnType<typeof targetRows> {
    const e = this.data.state.encounter();
    return rows.map((r) => {
      const c = e?.combatants.find((x) => x.id === r.id);
      const ally =
        c && (isPlayer(c) || c.side === CombatantSide.PARTY)
          ? article(c.label) === 'a'
            ? 'Aliada'
            : 'Aliado'
          : '';
      return ally ? { ...r, sub: tight(joinDots([r.sub, ally].filter(Boolean))) } : r;
    });
  }
  private readonly baseRows = computed(() =>
    this.data.opportunity
      ? [
          {
            id: this.data.opportunity.targetId,
            label: this.data.opportunity.targetLabel,
            sub: '',
            blocked: '',
            cover: '',
            coverMark: null,
          },
        ]
      : this.data.asReaction
        ? targetRows(
            this.data.targets.map(
              (t) => ({ ...t, tooFar: t.distanceFt === undefined || t.distanceFt > 5 }) as typeof t,
            ),
            5,
          )
        : targetRows(this.data.targets, this.attack.rangeFt),
  );
  protected readonly stepList = computed(() => steps(this.stage()));
  protected readonly target = computed(() => {
    const id = this.targetId();
    const row = this.rows().find((r) => r.id === id);
    return row ?? null;
  });
  protected readonly targetLabel = computed(
    () => this.target()?.label ?? this.data.resume?.targetLabel ?? '',
  );
  protected readonly doneLabel = this.data.opportunity ? 'Fechar' : 'Voltar à sua vez';
  protected readonly canApp = this.data.diceMode !== DiceMode.PHYSICAL;
  protected readonly canType = this.data.diceMode !== DiceMode.APP;
  protected readonly preferApp =
    effectivePreference(this.data.diceMode, this.data.preference) === DicePreference.APP;

  protected readonly outcome = computed(() => {
    const r = this.roll();
    // A roll a reaction holds (Palavras de Interrupção) has no result yet: the wait line says so instead.
    return r && !r.heldForReaction
      ? {
          word: outcomeWord(r.outcome),
          hit: isHit(r.outcome),
          crit: r.outcome === AttackOutcome.CRITICAL_HIT,
        }
      : null;
  });
  protected readonly d20Formula = computed(() => {
    const r = this.roll()?.d20;
    return r ? rollFormula(r) : '';
  });
  /** The dice of the damage still to roll: "2d6" (doubled on a critical hit). */
  protected readonly dice = computed(() => {
    const p = this.pending();
    return p
      ? {
          count: p.diceCount,
          sides: p.diceSides,
          bonus: p.bonus,
          name: diceName(p.diceCount, p.diceSides),
        }
      : null;
  });
  /** The damage when a feature adds dice to the critical (Crítico Brutal): the screens say the groups and the feature's name. */
  protected readonly brutal = computed(() => {
    const p = this.pending();
    return p && hasExtraDice(p) ? p : null;
  });
  /** "2d12 + 1d12 + 3" and "2d12 do crítico (dados dobrados) e 1d12 do Crítico Brutal (nível 9), mais 3 de modificador, de cortante.". */
  protected readonly brutalCard = computed(() => {
    const b = this.brutal();
    return b ? { sum: criticalSum(b), sentence: criticalSentence(b, b.damageTypePt) } : null;
  });
  protected readonly range = computed(() => {
    const b = this.brutal();
    if (b) {
      return typedRange(b);
    }
    const d = this.dice();
    return d ? sumRange(d.count, d.sides) : { min: 1, max: 1 };
  });
  protected readonly damageLine = computed(() => {
    const d = this.damage();
    return d?.roll ? damageFormula(d.roll, d.damageTypePt, extraDiceOf(d), d.criticalMax) : '';
  });
  /** The typed sum's live total with the groups, only for the damage with extra dice. */
  protected readonly typedFormula = computed(() => {
    const b = this.brutal();
    return b
      ? (sum: number) => ({ text: brutalTyped(b, sum), total: sum + b.criticalMax + b.bonus })
      : null;
  });
  protected readonly totalNote = computed(() => {
    const b = this.brutal();
    return b && b.damageTypePt ? `de ${b.damageTypePt}` : 'Dano total';
  });
  protected readonly damageAppLabel = computed(() => {
    const d = this.dice();
    return this.brutal() ? 'Rolar dano no app' : `Rolar ${d?.name ?? ''} no app`;
  });
  protected readonly after = computed(() => {
    const d = this.damage();
    if (!d) {
      return '';
    }
    const enc = this.data.state.encounter();
    const target = enc?.combatants.find((c) => c.id === d.targetId);
    return targetAfter(
      this.targetLabel(),
      !!target && isPlayer(target),
      d,
      target ? stateWord(target.state) : '',
    );
  });
  /** The d20 is rolled but a reaction holds the result: "Esperando o mestre. O resultado do seu ataque sai quando ele responder." */
  protected readonly held = computed(() => {
    const r = this.roll();
    const e = this.data.state.encounter();
    return r?.heldForReaction
      ? ((e ? reactionWait(e) : null) ?? {
          title: 'Esperando o mestre',
          detail: 'O resultado do seu ataque sai quando ele responder.',
        })
      : null;
  });
  /** The hit is made, but its damage waits for the target's reaction (Escudo). */
  protected readonly waiting = computed(() => awaitsReaction(this.pending()));
  protected readonly defeated = computed(() => this.damage()?.targetDefeated ?? false);
  /** What the attack spent: the reaction, one of Extra Attack's attacks (the
   * action stays open for the rest) or the action. */
  protected readonly spent = computed(() => {
    if (this.data.opportunity?.byMaster) {
      return 'A reação dele foi usada.';
    }
    if (this.data.asReaction) {
      return 'Sua reação foi usada.';
    }
    const bonus = bonusSpentText(this.data.bonusRule, this.data.bonusAttacksLeft);
    if (bonus) {
      return bonus;
    }
    if (this.cantrip && (this.data.beamsLeft ?? 0) > 1) {
      return beamSpentText(this.name, this.data.beamsLeft ?? 0);
    }
    const left = (this.data.attacksLeft ?? 1) - 1;
    if (!this.cantrip && (this.data.attacksPerAction ?? 1) > 1 && left > 0) {
      return `Você ainda tem ${left} ${left === 1 ? 'ataque' : 'ataques'} desta ação.`;
    }
    return 'Sua ação foi usada.';
  });
  protected readonly title = computed(() =>
    this.typing() ? 'Digite o resultado do dado' : `Atacar com ${this.name}`,
  );
  protected readonly subtitle = computed(() =>
    this.typing()
      ? `${this.name} contra ${this.targetLabel()} · Rodada ${this.data.round}`
      : this.detail,
  );
  protected readonly rollLabel = computed(
    () => `Role 1d20 para ${this.name} (${this.signedBonus()})`,
  );
  protected readonly damageLabel = computed(() => {
    const b = this.brutal();
    if (b) {
      return brutalLabel(b, `${article(this.name)} ${this.name}`);
    }
    const d = this.dice();
    return d && d.count > 1
      ? `Role ${d.name} para o dano: some os dois`
      : `Role ${d?.name ?? ''} para o dano`;
  });
  protected readonly damageHint = computed(() => {
    const b = this.brutal();
    if (b) {
      return brutalTypedHint(b);
    }
    const r = this.range();
    const p = this.pending();
    return criticalTypedHint(
      p?.criticalRule ?? 0,
      p ? diceName(p.diceCount, p.diceSides) : '',
      r.min,
      r.max,
      p?.criticalMax ?? 0,
    );
  });
  /** What the typed sum is added to: the modifier and the critical's fixed maximum (the server's numbers, shown in the total before it is sent). */
  protected readonly damageModifier = computed(
    () => (this.pending()?.bonus ?? 0) + (this.pending()?.criticalMax ?? 0),
  );
  protected readonly fixedText = computed(() => {
    const b = this.brutal();
    if (b) {
      // The maximum stands in the live total's groups; the field's note is the modifier alone.
      return b.bonus === 0 ? '' : `${b.bonus < 0 ? '−' : '+'} ${Math.abs(b.bonus)} de bônus`;
    }
    return fixedParts(this.pending()?.criticalMax ?? 0, this.pending()?.bonus ?? 0);
  });
  /** The critical's line only for whoever rolls physical dice (the app's dice are the server's). */
  protected readonly showCritical = computed(() => !this.canApp || this.typing());
  /** The line above the damage's dice: what a critical hit rolls under the table's rule ("role os dados duas vezes", "o máximo mais uma rolagem"), or "Acertou: role o dano.". */
  protected readonly damageIntro = computed(() => {
    const p = this.pending();
    if (!p) {
      return '';
    }
    const hint = criticalHint(p.criticalRule, p.diceCount, p.diceSides, p.criticalMax);
    if (hint) {
      return hint.line;
    }
    return p.critical || this.outcome()?.crit
      ? `Acerto crítico: os dados do dano dobram (${diceName(p.diceCount, p.diceSides)}).`
      : 'Acertou: role o dano.';
  });
  /** A combat without a map: the master judges the reach, and the list says so instead of "Longe demais". */
  protected readonly theatre = computed(() => isTheatre(this.data.state.encounter()));

  constructor() {
    // After a result the focus goes to the one next action, as soon as it is drawn.
    effect(() => this.back()?.nativeElement.focus());
    // The answer of a request in the air has to be shown: the sheet can't be dismissed meanwhile.
    effect(() => this.sheet.lock(this.busy()));
    // An error opens at the top of the scrolling body, where it is seen.
    effect(() => {
      if (this.error()) {
        this.body()?.nativeElement.scrollTo({ top: 0 });
      }
    });
  }

  private signedBonus(): string {
    const b = this.attack.attackBonus;
    return `${b < 0 ? '−' : '+'}${Math.abs(b)}`;
  }

  protected initial(label: string): string {
    return combatantInitial(label);
  }

  protected npcTarget(id: string): boolean {
    const c = this.data.state.encounter()?.combatants.find((x) => x.id === id);
    return !!c && !isPlayer(c) && !isCreature(c);
  }

  protected creatureTarget(id: string): boolean {
    const c = this.data.state.encounter()?.combatants.find((x) => x.id === id);
    return !!c && isCreature(c);
  }

  protected pick(id: string): void {
    this.targetId.set(id);
    this.error.set('');
  }

  /** The target is chosen: on to the roll (a click, or Enter). */
  protected advance(): void {
    if (this.targetId() !== null) {
      this.stage.set('roll');
    }
  }

  protected change(): void {
    this.typing.set(false);
    this.stage.set('target');
  }

  protected async rollAttack(die: AttackDie): Promise<void> {
    const id = this.targetId();
    if (id === null || this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      const res = await this.api.rollAttack(
        this.data.campaignId,
        this.data.encounterId,
        this.data.attackerId,
        this.attack.key,
        id,
        die,
        this.attackKeys.keyFor({ id, die }),
        this.data.asReaction ?? false,
        this.data.opportunity?.offerId ?? '',
        this.data.catchWindowId ?? '',
      );
      this.data.state.apply(res.encounter);
      this.roll.set(res.roll);
      this.pending.set(res.pending ?? null);
      this.typing.set(false);
      this.stage.set(
        res.roll.heldForReaction ? 'done' : stageAfterRoll(res.roll.outcome, res.pending),
      );
    } catch (err) {
      this.fail(err);
    } finally {
      this.busy.set(false);
    }
  }

  protected rollAttackTyped(face: number): Promise<void> {
    return this.rollAttack({ face });
  }

  protected async rollDamage(die: DamageDie): Promise<void> {
    const p = this.pending();
    if (!p || this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      const res = await this.api.rollDamage(
        this.data.campaignId,
        this.data.encounterId,
        p.id,
        die,
        this.damageKeys.keyFor(die),
      );
      this.data.state.apply(res.encounter);
      this.damage.set(res.pending);
      this.typing.set(false);
      this.stage.set('done');
    } catch (err) {
      this.fail(err);
    } finally {
      this.busy.set(false);
    }
  }

  protected rollDamageTyped(sum: number): Promise<void> {
    return this.rollDamage({ sum });
  }

  private fail(err: unknown): void {
    this.error.set(combatErrorMessage(err, 'rolar o ataque'));
  }

  protected close(): void {
    if (this.busy()) {
      return;
    }
    this.sheet.close(this.stage() === 'done');
  }
}
