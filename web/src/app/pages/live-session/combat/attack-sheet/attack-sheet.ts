import {
  Component,
  ElementRef,
  computed,
  effect,
  inject,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
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
import {
  type RollModeRequest,
  RollMode,
  RollModeRequestStatus,
} from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
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
  type ModeChoice,
  type PairDie,
  CombatClient,
} from '../../../../core/combat/combat-client';
import { ActionKey } from '../../../../core/connect/idempotency';
import { damageFormula, diceName, sumRange } from '../../../../core/combat/combat-dice';
import { criticalHint, criticalTypedHint, fixedParts } from '../../../../core/combat/critical';
import {
  type ExtraPick,
  activeParts,
  initialPicks,
  typedFields,
} from '../../../../core/combat/damage-parts';
import {
  d20Count,
  d20Formula,
  liveRequest,
  modeStatus,
  orNormal,
} from '../../../../core/combat/roll-mode';
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
import { DamageParts } from '../damage-parts/damage-parts';
import { MultiRoll, type RollField } from '../multi-roll/multi-roll';
import { RollModePicker } from '../roll-mode/roll-mode-picker';
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
  /** The master rolls it (for an NPC): any roll mode, with no need to ask. */
  readonly master?: boolean;
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
  imports: [
    AttackResult,
    AttackSteps,
    CombatantToken,
    DamageParts,
    MatButtonModule,
    MatIconModule,
    MultiRoll,
    RollModePicker,
    RollPicker,
  ],
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
  /** The mode the player picked; null follows the server's suggestion. */
  protected readonly picked = signal<RollMode | null>(null);
  protected readonly reason = signal('');
  /** The request this sheet sent to the master (its state is read from the combat). */
  private readonly asked = signal<RollModeRequest | null>(null);
  /** The extras marked in the damage step. */
  protected readonly picks = signal<readonly ExtraPick[]>([]);

  /** One key per attack roll (target and die): the same values again are a retry, others a new attack. */
  private readonly attackKeys = new ActionKey();
  /** One key per damage roll: the same die again is a retry, another die (typed after an app roll was lost) is a new request. */
  private readonly damageKeys = new ActionKey();
  /** One key per request to the master: the same mode and reason again is a retry. */
  private readonly requestKeys = new ActionKey();
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
  /** The target's circumstances from the options: the suggested mode and why. An opportunity attack has none. */
  private readonly reach = computed(() => {
    const id = this.targetId();
    return this.data.targets.find((t) => t.combatantId === id) ?? null;
  });
  protected readonly hasMode = computed(() => !this.data.opportunity && this.reach() !== null);
  protected readonly suggested = computed(() =>
    orNormal(this.reach()?.rollMode ?? RollMode.NORMAL),
  );
  protected readonly sources = computed(() => this.reach()?.sources ?? []);
  protected readonly criticalOnHit = computed(() => this.reach()?.criticalOnHit ?? false);
  protected readonly approval = this.data.master ? 'free' : 'request';
  protected readonly chosen = computed(() => this.picked() ?? this.suggested());
  /** The request of this attack as the combat shows it now (waiting, answered), or none. */
  protected readonly request = computed(() => {
    const sent = this.asked();
    if (!sent) {
      return null;
    }
    const all = this.data.state.encounter()?.rollModeRequests ?? [];
    const live = liveRequest(all, sent.id);
    return live ?? (all.some((r) => r.id === sent.id) ? null : sent);
  });
  private readonly answered = computed(() => {
    const r = this.request();
    return r?.status === RollModeRequestStatus.ANSWERED ? r : null;
  });
  /** The mode the d20 rolls with: the master's decision on a request, else the player's choice. */
  protected readonly rollMode = computed(() => this.answered()?.decidedMode ?? this.chosen());
  protected readonly modeState = computed(() =>
    modeStatus(this.approval, this.suggested(), this.chosen(), this.reason(), this.request()),
  );
  /** The dice may be rolled: the mode is settled (or the server picks it, for an opportunity attack). */
  protected readonly modeReady = computed(() => !this.hasMode() || this.modeState() === 'ready');
  protected readonly faceCount = computed(() => (this.hasMode() ? d20Count(this.rollMode()) : 1));
  protected readonly d20Fields = computed<readonly RollField[]>(() => [
    { key: 'd20-1', label: 'Primeiro d20', min: 1, max: 20 },
    { key: 'd20-2', label: 'Segundo d20', min: 1, max: 20 },
  ]);
  protected readonly combine = computed(() =>
    this.rollMode() === RollMode.DISADVANTAGE ? 'lower' : 'higher',
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
    return r
      ? {
          word: outcomeWord(r.outcome),
          hit: isHit(r.outcome),
          crit: r.outcome === AttackOutcome.CRITICAL_HIT,
        }
      : null;
  });
  protected readonly d20Formula = computed(() => {
    const r = this.roll()?.d20;
    return r ? d20Formula(r) : '';
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
  protected readonly range = computed(() => {
    const d = this.dice();
    return d ? sumRange(d.count, d.sides) : { min: 1, max: 1 };
  });
  protected readonly parts = computed(() => this.pending()?.parts ?? []);
  /** One field for each group of dice of the damage that is marked, for physical dice. */
  protected readonly damageFields = computed<readonly RollField[]>(() =>
    typedFields(this.parts(), this.picks()).map((f) => ({
      key: f.key,
      label: f.label,
      min: f.min,
      max: f.max,
    })),
  );
  protected readonly damageFlat = computed(() =>
    activeParts(this.parts(), this.picks()).reduce((sum, p) => sum + p.flat, 0),
  );
  protected readonly damageLine = computed(() => {
    const d = this.damage();
    return d?.roll ? damageFormula(d.roll, d.damageTypePt) : '';
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
    const d = this.dice();
    return d && d.count > 1
      ? `Role ${d.name} para o dano: some os dois`
      : `Role ${d?.name ?? ''} para o dano`;
  });
  protected readonly damageHint = computed(() => {
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
  protected readonly fixedText = computed(() =>
    fixedParts(this.pending()?.criticalMax ?? 0, this.pending()?.bonus ?? 0),
  );
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
    // The extras that come marked are the starting choice of each damage.
    effect(() => {
      const parts = this.pending()?.parts ?? [];
      untracked(() => this.picks.set(initialPicks(parts)));
    });
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
    this.dropMode();
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
    this.dropMode();
  }

  /** A new target starts from its own suggestion; a request for the old one is taken back. */
  private dropMode(): void {
    const sent = this.asked();
    this.picked.set(null);
    this.reason.set('');
    this.asked.set(null);
    if (sent && sent.status === RollModeRequestStatus.PENDING) {
      void this.api
        .cancelRollModeRequest(
          this.data.campaignId,
          this.data.encounterId,
          sent.id,
          this.requestKeys.keyFor({ cancel: sent.id }),
        )
        .then((e) => this.data.state.apply(e))
        .catch(() => undefined);
    }
  }

  protected setMode(mode: RollMode): void {
    this.picked.set(mode);
    this.error.set('');
  }

  /** "Pedir ao mestre": the mode asked for and why; the sheet waits for the answer. */
  protected async askMaster(): Promise<void> {
    const id = this.targetId();
    if (id === null || this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      const mode = this.chosen();
      const reason = this.reason().trim();
      const res = await this.api.requestRollMode(
        this.data.campaignId,
        this.data.encounterId,
        this.data.attackerId,
        this.attack.key,
        id,
        mode,
        reason,
        this.requestKeys.keyFor({ id, mode, reason }),
      );
      this.data.state.apply(res.encounter);
      this.asked.set(res.request);
    } catch (err) {
      this.error.set(combatErrorMessage(err, 'pedir ao mestre'));
    } finally {
      this.busy.set(false);
    }
  }

  /** "Cancelar pedido": the player takes it back and picks again. */
  protected async cancelAsk(): Promise<void> {
    const sent = this.asked();
    if (!sent || this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      const e = await this.api.cancelRollModeRequest(
        this.data.campaignId,
        this.data.encounterId,
        sent.id,
        this.requestKeys.keyFor({ cancel: sent.id }),
      );
      this.data.state.apply(e);
      this.asked.set(null);
      this.picked.set(null);
      this.reason.set('');
    } catch (err) {
      this.error.set(combatErrorMessage(err, 'cancelar o pedido'));
    } finally {
      this.busy.set(false);
    }
  }

  /** What the roll sends about the mode: nothing when the suggestion stands, else the mode, why, and the answered request. */
  private modeChoice(): ModeChoice | undefined {
    if (!this.hasMode()) {
      return undefined;
    }
    const r = this.answered();
    if (r) {
      return {
        mode: r.decidedMode,
        reason: r.decidedMode === r.suggestedMode ? '' : r.reason,
        requestId: r.id,
      };
    }
    return this.chosen() === this.suggested()
      ? undefined
      : { mode: this.chosen(), reason: this.reason().trim() };
  }

  protected async rollAttack(die: AttackDie | PairDie): Promise<void> {
    const id = this.targetId();
    if (id === null || this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      const mode = this.modeChoice();
      const res = await this.api.rollAttack(
        this.data.campaignId,
        this.data.encounterId,
        this.data.attackerId,
        this.attack.key,
        id,
        die,
        this.attackKeys.keyFor({ id, die, mode }),
        this.data.asReaction ?? false,
        this.data.opportunity?.offerId ?? '',
        mode,
      );
      this.data.state.apply(res.encounter);
      this.roll.set(res.roll);
      this.pending.set(res.pending ?? null);
      this.typing.set(false);
      this.stage.set(stageAfterRoll(res.roll.outcome, res.pending));
    } catch (err) {
      this.fail(err);
    } finally {
      this.busy.set(false);
    }
  }

  protected rollAttackTyped(face: number): Promise<void> {
    return this.rollAttack({ face });
  }

  /** Two physical d20, in the order they were rolled. */
  protected rollAttackPair(faces: number[]): Promise<void> {
    return this.rollAttack({ faces });
  }

  protected async rollDamage(
    die: DamageDie | { readonly parts: true },
    typedParts?: { partKey: string; sum: number }[],
  ): Promise<void> {
    const p = this.pending();
    if (!p || this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      // With parts the answer is the whole choice, the extras marked (even none) and the sums typed.
      const choice =
        this.parts().length > 0
          ? { extras: this.picks(), ...(typedParts ? { typedParts } : {}) }
          : undefined;
      const res = await this.api.rollDamage(
        this.data.campaignId,
        this.data.encounterId,
        p.id,
        die,
        this.damageKeys.keyFor({ die, choice }),
        choice,
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

  /** The sums typed for each group of dice, in the order of the fields. */
  protected rollDamageParts(sums: number[]): Promise<void> {
    const fields = this.damageFields();
    return this.rollDamage(
      { parts: true },
      fields.map((f, i) => ({ partKey: f.key, sum: sums[i] })),
    );
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
