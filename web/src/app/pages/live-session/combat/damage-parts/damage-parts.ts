import { ChangeDetectionStrategy, Component, computed, input, model } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { DamagePart, SlotOption } from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import {
  type ExtraPick,
  defaultSlot,
  partDice,
  partFormula,
} from '../../../../core/combat/damage-parts';

let nextId = 0;

/**
 * What goes into the damage of a hit, before it is rolled: the lines that come
 * with it (the weapon, Fúria, Duelismo) listed as fixed, and the extras the
 * player may add (Ataque Furtivo, Golpe Divino, Marca do Caçador) as checkboxes.
 * An extra whose condition fails is disabled and says why. Golpe Divino asks for
 * the slot it spends. On a critical hit the dice parts say "dados dobrados"; the
 * fixed numbers are not doubled. Marking is the player's answer: even none counts.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-damage-parts',
  imports: [MatIconModule],
  templateUrl: './damage-parts.html',
  styleUrl: './damage-parts.scss',
})
export class DamageParts {
  readonly parts = input.required<readonly DamagePart[]>();
  /** The extras marked, each with the slot it spends. */
  readonly picks = model<readonly ExtraPick[]>([]);
  readonly disabled = input(false);

  protected readonly id = `damage-parts-${nextId++}`;
  protected readonly fixed = computed(() => this.parts().filter((p) => !p.choosable));
  protected readonly extras = computed(() => this.parts().filter((p) => p.choosable));

  /** Another maneuver is marked: only one maneuver for each attack, so this one is off until it is unmarked. */
  protected otherManeuver(p: DamagePart): boolean {
    return (
      p.maneuver &&
      !this.pickOf(p) &&
      this.parts().some((o) => o.maneuver && o.key !== p.key && !!this.pickOf(o))
    );
  }

  protected pickOf(p: DamagePart): ExtraPick | undefined {
    return this.picks().find((x) => x.key === p.key);
  }

  protected formula(p: DamagePart): string {
    return partFormula(p, this.pickOf(p));
  }

  protected doubled(p: DamagePart): boolean {
    return p.doubled && partDice(p, this.pickOf(p)).count > 0;
  }

  protected toggle(p: DamagePart): void {
    if (this.pickOf(p)) {
      this.picks.set(this.picks().filter((x) => x.key !== p.key));
      return;
    }
    const slot = p.needsSlot ? defaultSlot(p) : null;
    this.picks.set([
      ...this.picks(),
      { key: p.key, slotLevel: slot?.level ?? 0, pact: slot?.pact ?? false },
    ]);
  }

  protected chooseSlot(p: DamagePart, o: SlotOption): void {
    this.picks.set(
      this.picks().map((x) => (x.key === p.key ? { ...x, slotLevel: o.level, pact: o.pact } : x)),
    );
  }

  protected slotOn(p: DamagePart, o: SlotOption): boolean {
    const pick = this.pickOf(p);
    return !!pick && pick.slotLevel === o.level && pick.pact === o.pact;
  }

  protected slotText(p: DamagePart, o: SlotOption): string {
    const name = o.pact ? `Pacto, ${o.level}º nível` : `${o.level}º nível`;
    return `${name} · ${o.free} de ${o.max} ${o.max === 1 ? 'livre' : 'livres'} · ${o.diceCount}d${p.diceSides}`;
  }
}
