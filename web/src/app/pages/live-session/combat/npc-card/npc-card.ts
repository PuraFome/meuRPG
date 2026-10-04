import { Component, computed, effect, inject, input, output, signal, untracked, viewChild } from '@angular/core';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatSelectModule } from '@angular/material/select';
import { MatIconModule } from '@angular/material/icon';

import {
  type AttackRoll,
  type Combatant,
  type Encounter,
  type GetTurnOptionsResponse,
  type PendingDamage,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import type { Attack } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { isHit, outcomeWord } from '../../../../core/combat/attack-flow';
import { CombatClient, newKey } from '../../../../core/combat/combat-client';
import { rollFormula } from '../../../../core/combat/combat-dice';
import { article } from '../../../../core/combat/combat-log';
import { combatErrorMessage } from '../../../../core/combat/combat-errors';
import { distanceText, metersText, reachSquares, squaresText } from '../../../../core/units';
import type { CombatState } from '../../../../core/combat/combat-state';
import { attackName } from '../../../../core/combat/combat-options';
import { combatantInitial, isPlayer, roundLabel } from '../../../../core/combat/combat-view';
import { CombatantToken } from '../../../../shared/combatant-token/combatant-token';
import { NextTurn } from '../combat-bar/next-turn';
import { RollPicker } from '../roll-picker/roll-picker';
import { AttackChoice } from './attack-choice';
import { PendingDamages } from './pending-damages';

/**
 * The master's card of the one on turn (E6-11, E6-12): "Ações do Capitão
 * Goblin" with its PV, CA and speed, the economy it spent, the attack (radio
 * rows), the target with the distance and "Rolar ataque" (the app rolls it,
 * or the master types a value). The result shows the armor class the roll
 * was compared with ("Acertou contra CA 18 do Toren"), then the damage to
 * roll and apply (`PendingDamages`). The one filled button of the screen
 * stays "Próximo turno". When a player is on turn the card has no attack
 * form, only the damage their hits left for the master to apply.
 */
@Component({
  selector: 'app-npc-card',
  imports: [AttackChoice, CombatantToken, MatFormFieldModule, MatIconModule, MatSelectModule, NextTurn, PendingDamages, RollPicker],
  templateUrl: './npc-card.html',
  styleUrl: './npc-card.scss',
})
export class NpcCard {
  private readonly api = inject(CombatClient);

  readonly campaignId = input.required<string>();
  readonly encounter = input.required<Encounter>();
  /** The one on turn (a player's character too: only its pending damage). */
  readonly subject = input.required<Combatant>();
  readonly options = input<GetTurnOptionsResponse | null>(null);
  readonly state = input.required<CombatState>();
  /** On a phone or a tablet the card is the whole turn (E6-12): its title, the
   * warning that damage is owed and "Próximo turno" at its end. */
  readonly narrow = input(false);
  /** What the turn still owes ("Falta aplicar 5 de dano"), or `null`. */
  readonly pendingNote = input<string | null>(null);
  /** The newest attack of this one was stopped by the target's Escudo (from the log). */
  readonly reactionStopped = input(false);
  /** The page's own call (the turn passing) is in flight. */
  readonly turnBusy = input(false);
  /** "Próximo turno"; `true` when the master passes it with a damage waiting. */
  readonly next = output<boolean>();
  /** A damage was applied or discarded (`PendingDamages`): the page keeps the card for its note. */
  readonly settledNote = output<void>();

  protected readonly attackKey = signal('');
  protected readonly targetId = signal('');
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  /** The master is typing the d20: the roll takes the whole row. */
  protected readonly typing = signal(false);
  private readonly picker = viewChild(RollPicker);
  /** The master's own "Usar Escudo por ele" stopped the hit. */
  private readonly stoppedHere = signal(false);
  /** The d20 just rolled: shown with the armor class, until the turn changes. */
  protected readonly last = signal<{ roll: AttackRoll; pending: PendingDamage | null; subject: string } | null>(null);
  private key = newKey();
  /** The reaction spell's name, remembered from the prompt that waited (it is gone once answered). */
  protected readonly shieldName = signal('Escudo Arcano');

  protected readonly round = computed(() => roundLabel(this.encounter().round));
  protected readonly initial = computed(() => combatantInitial(this.subject().label));
  protected readonly isNpc = computed(() => !isPlayer(this.subject()));
  protected readonly attacks = computed<Attack[]>(() =>
    (this.options()?.options?.attacks ?? []).flatMap((a) => (a.attack && a.attack.saveDc === 0 ? [a.attack] : [])),
  );
  protected readonly attack = computed(() => this.attacks().find((a) => a.key === this.attackKey()) ?? null);
  protected readonly targets = computed(
    () => this.options()?.attackTargets.find((t) => t.attackKey === this.attackKey())?.targets ?? [],
  );
  protected readonly targetLabel = computed(
    () => this.targets().find((t) => t.combatantId === this.targetId())?.label ?? '',
  );
  protected readonly pendings = computed(() => this.options()?.pendingDamages ?? []);
  /** The damage of the attack just rolled shows inside its result box. */
  protected readonly inBox = computed(() => {
    const id = this.last()?.pending?.id;
    return this.pendings().filter((p) => p.id === id);
  });
  protected readonly others = computed(() => {
    const id = this.last()?.pending?.id;
    return this.pendings().filter((p) => p.id !== id);
  });
  protected readonly stats = computed(() => {
    const c = this.subject();
    return {
      hp: c.hitPointsMax !== undefined ? { now: c.hitPointsCurrent ?? 0, max: c.hitPointsMax } : null,
      ac: c.armorClass,
      speed: { meters: metersText(c.speedFt), squares: squaresText(reachSquares(c.speedFt)) },
      movement: distanceText(c.movementLeftFt),
    };
  });
  protected readonly result = computed(() => {
    const l = this.last();
    if (!l) {
      return null;
    }
    const target = this.encounter().combatants.find((c) => c.id === l.roll.targetId);
    // Escudo can turn a hit into a miss after the roll (the master's answer, or the player's).
    const stopped = this.stoppedHere() || this.reactionStopped();
    return {
      total: l.roll.d20?.total ?? 0,
      formula: l.roll.d20 ? rollFormula(l.roll.d20) : '',
      physical: l.roll.d20?.physical ?? false,
      word: stopped ? `Errou: o ${this.shieldName()} segurou` : outcomeWord(l.roll.outcome),
      hit: !stopped && isHit(l.roll.outcome),
      against:
        l.roll.targetArmorClass !== undefined
          ? `contra CA ${l.roll.targetArmorClass} ${article(target?.label ?? '') === 'a' ? 'da' : 'do'} ${target?.label ?? ''}`
          : '',
    };
  });
  protected readonly rollLabel = computed(() => {
    const a = this.attack();
    return a ? `Role 1d20 para ${attackName(a)} (${a.attackBonus < 0 ? '−' : '+'}${Math.abs(a.attackBonus)})` : '';
  });


  constructor() {
    // Each new turn starts clean, with the first attack and the first target.
    let turnOf = '';
    effect(() => {
      const id = this.subject().id;
      if (id !== turnOf) {
        turnOf = id;
        untracked(() => {
          this.last.set(null);
          this.error.set('');
        });
      }
    });
    effect(() => {
      const prompt = this.encounter().reactionPrompts[0];
      if (prompt?.spellNamePt) {
        this.shieldName.set(prompt.spellNamePt);
      }
    });
    effect(() => {
      const keys = this.attacks().map((a) => a.key);
      if (!keys.includes(this.attackKey())) {
        this.attackKey.set(keys[0] ?? '');
      }
    });
    effect(() => {
      const ids = this.targets().map((t) => t.combatantId);
      if (!ids.includes(this.targetId())) {
        this.targetId.set(ids[0] ?? '');
      }
    });
  }

  protected onReacted(what: 'stopped' | 'still' | 'declined'): void {
    this.stoppedHere.set(what === 'stopped');
  }

  protected pickAttack(key: string): void {
    this.attackKey.set(key);
    this.key = newKey();
  }

  protected pickTarget(id: string): void {
    this.targetId.set(id);
    this.key = newKey();
  }

  protected distance(ft: number | undefined, tooFar: boolean): string {
    const parts: string[] = [];
    if (ft !== undefined) {
      parts.push(`a ${metersText(ft)}`);
    }
    if (tooFar) {
      parts.push('fora do alcance');
    }
    return parts.join(', ');
  }

  protected rollApp(): Promise<void> {
    return this.rollAttack({ inApp: true });
  }

  protected rollTyped(face: number): Promise<void> {
    return this.rollAttack({ face });
  }

  private async rollAttack(die: { inApp: true } | { face: number }): Promise<void> {
    const a = this.attack();
    const target = this.targetId();
    if (!a || !target || this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      const res = await this.api.rollAttack(
        this.campaignId(),
        this.encounter().id,
        this.subject().id,
        a.key,
        target,
        die,
        this.key,
      );
      this.key = newKey();
      this.state().apply(res.encounter);
      this.last.set({ roll: res.roll, pending: res.pending ?? null, subject: this.subject().id });
      this.picker()?.reset();
      this.stoppedHere.set(false);
    } catch (err) {
      this.error.set(combatErrorMessage(err, 'rolar o ataque'));
    } finally {
      this.busy.set(false);
    }
  }
}
