import { Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { Encounter } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { missingInitiative, tieGroups } from '../../../../core/combat/combat-view';
import type { CombatantInfo } from '../combat-info';

/**
 * "Falta uma rolagem" (E6-04): what stands between the master and the first
 * turn. The button that starts it, "Começar o combate" (the page's one filled button), and "Cancelar combate" are
 * in the sticky combat bar (`CombatBar`); this card keeps the reason the button points at with `aria-describedby`.
 * A tie does not block the start: the order stays as it is (BeginCombat).
 */
@Component({
  selector: 'app-initiative-side',
  imports: [MatIconModule],
  templateUrl: './initiative-side.html',
  styleUrl: './initiative-side.scss',
})
export class InitiativeSide {
  readonly encounter = input.required<Encounter>();
  readonly info = input<ReadonlyMap<string, CombatantInfo>>(new Map());
  protected readonly total = computed(() => this.encounter().combatants.length);
  protected readonly missing = computed(() => missingInitiative(this.encounter().combatants));
  protected readonly rolled = computed(() => this.total() - this.missing().length);
  protected readonly ties = computed(() => tieGroups(this.encounter().combatants));
  protected readonly tieLines = computed(() =>
    this.ties().map((group) => {
      const members = group.map((id) => this.encounter().combatants.find((c) => c.id === id));
      const names = members.map((c) => c?.label ?? '').join(' e ');
      return `Empate em ${members[0]?.initiative}: escolha a ordem de ${names}`;
    }),
  );
  protected readonly ready = computed(() => this.missing().length === 0);

  /** "Toren", "Toren e Brisa", "Toren, Brisa e Goblin 1": no article, since
   * a name does not say its gender. */
  protected readonly names = computed(() => {
    const labels = this.missing().map((c) => c.label);
    return labels.length <= 1
      ? (labels[0] ?? '')
      : `${labels.slice(0, -1).join(', ')} e ${labels[labels.length - 1]}`;
  });
  protected readonly reason = computed(() =>
    this.missing().length === 1
      ? `Falta a iniciativa de ${this.names()}.`
      : `Faltam as iniciativas de ${this.names()}.`,
  );
  /** Who the master waits for, by the player's name when there is one. */
  protected readonly waiting = computed(() => {
    const [only] = this.missing();
    if (this.missing().length !== 1 || !only) {
      return null;
    }
    const player = this.info().get(only.characterId)?.playerName;
    return player ? `${player} rola os próprios dados.` : null;
  });
  protected readonly title = computed(() =>
    this.ready()
      ? 'Tudo pronto'
      : this.missing().length === 1
        ? 'Falta uma rolagem'
        : 'Faltam rolagens',
  );
}
