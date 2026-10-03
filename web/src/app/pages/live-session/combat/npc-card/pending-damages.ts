import { Component, ElementRef, Injector, afterNextRender, computed, inject, input, signal, viewChild } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import {
  type Encounter,
  type PendingDamage,
  PendingDamageStatus,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { CombatClient, newKey } from '../../../../core/combat/combat-client';
import { damageFormula, diceName, sumRange } from '../../../../core/combat/combat-dice';
import { combatErrorMessage } from '../../../../core/combat/combat-errors';
import type { CombatState } from '../../../../core/combat/combat-state';
import { hitPointsAfter, hitPointsLine } from '../../../../core/combat/attack-flow';
import { isPlayer } from '../../../../core/combat/combat-view';
import { RollPicker } from '../roll-picker/roll-picker';

/**
 * The damage the master owes a hit (E6-11, E6-12, RN-02): "Rolar dano" for
 * an NPC's hit whose damage is not rolled yet, and, once rolled on a
 * player's character, the damage with the target's hit points ("Toren: 26
 * de 31 PV, depois 21") and "Aplicar 5 de dano" / "Não aplicar". It lists
 * every pending damage of the turn, also those a player rolled. "Aplicar"
 * is a 2px accent outline, because "Próximo turno" owns the fill. "Não
 * aplicar" asks in place ("Descartar o dano de 5?"), the focus on "Voltar".
 * The page's `CombatState` gets each answer's combat.
 */
@Component({
  selector: 'app-pending-damages',
  imports: [MatButtonModule, MatIconModule, RollPicker],
  templateUrl: './pending-damages.html',
  styleUrl: './pending-damages.scss',
})
export class PendingDamages {
  private readonly api = inject(CombatClient);
  private readonly injector = inject(Injector);

  readonly pendings = input.required<readonly PendingDamage[]>();
  readonly encounter = input.required<Encounter>();
  readonly campaignId = input.required<string>();
  readonly state = input.required<CombatState>();
  /** Inside the attack's own result box: no box of its own. */
  readonly flat = input(false);

  protected readonly busy = signal(false);
  protected readonly error = signal('');
  protected readonly discarding = signal<string | null>(null);
  /** What was just settled, for the live region ("Dano de 5 aplicado…"). */
  protected readonly settled = signal('');
  private readonly keys = new Map<string, string>();
  private readonly safe = viewChild('safe', { read: ElementRef<HTMLButtonElement> });

  protected readonly items = computed(() =>
    this.pendings()
      .filter((p) => p.status === PendingDamageStatus.ROLLED || p.status === PendingDamageStatus.AWAITING_ROLL)
      .map((p) => this.describe(p)),
  );

  private describe(p: PendingDamage) {
    const e = this.encounter();
    const attacker = e.combatants.find((c) => c.id === p.attackerId);
    const target = e.combatants.find((c) => c.id === p.targetId);
    const rolled = p.status === PendingDamageStatus.ROLLED;
    const range = sumRange(p.diceCount, p.diceSides);
    return {
      p,
      rolled,
      // A player rolls the damage of their own attack; the master waits.
      waitsForPlayer: !rolled && !!attacker && isPlayer(attacker),
      attacker: attacker?.label ?? '',
      target: target?.label ?? '',
      dice: diceName(p.diceCount, p.diceSides),
      range,
      label: p.diceCount > 1 ? `Role ${diceName(p.diceCount, p.diceSides)} para o dano: some os dois` : `Role ${diceName(p.diceCount, p.diceSides)} para o dano`,
      line: p.roll ? damageFormula(p.roll, p.damageTypePt) : '',
      effect:
        rolled && target && target.hitPointsMax !== undefined
          ? hitPointsLine(
              target.label,
              target.hitPointsCurrent ?? 0,
              target.hitPointsMax,
              hitPointsAfter(target.hitPointsCurrent ?? 0, target.hitPointsTemporary ?? 0, p.amount),
            )
          : '',
    };
  }

  private keyFor(id: string): string {
    let key = this.keys.get(id);
    if (!key) {
      key = newKey();
      this.keys.set(id, key);
    }
    return key;
  }

  private async run(work: () => Promise<void>): Promise<void> {
    if (this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      await work();
    } catch (err) {
      this.error.set(combatErrorMessage(err, 'resolver o dano'));
    } finally {
      this.busy.set(false);
    }
  }

  protected rollInApp(p: PendingDamage): Promise<void> {
    return this.rolled(p, { inApp: true });
  }

  protected rollTyped(p: PendingDamage, sum: number): Promise<void> {
    return this.rolled(p, { sum });
  }

  private rolled(p: PendingDamage, die: { inApp: true } | { sum: number }): Promise<void> {
    return this.run(async () => {
      const res = await this.api.rollDamage(this.campaignId(), this.encounter().id, p.id, die, this.keyFor(p.id));
      this.state().apply(res.encounter);
    });
  }

  protected apply(p: PendingDamage): Promise<void> {
    return this.run(async () => {
      const res = await this.api.applyDamage(this.campaignId(), this.encounter().id, p.id);
      this.state().apply(res.encounter);
      this.settled.set(`Dano de ${p.amount} aplicado.`);
    });
  }

  protected askDiscard(id: string): void {
    this.discarding.set(id);
    afterNextRender(() => this.safe()?.nativeElement.focus(), { injector: this.injector });
  }

  protected discard(p: PendingDamage): Promise<void> {
    return this.run(async () => {
      const res = await this.api.discardDamage(this.campaignId(), this.encounter().id, p.id);
      this.state().apply(res.encounter);
      this.discarding.set(null);
      this.settled.set(`Dano de ${p.amount} descartado.`);
    });
  }
}
