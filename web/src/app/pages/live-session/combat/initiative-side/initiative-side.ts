import { Component, ElementRef, computed, effect, input, output, signal, viewChild } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { Encounter } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { missingInitiative, tieGroups } from '../../../../core/combat/combat-view';
import type { CombatantInfo } from '../combat-info';

/**
 * "Falta uma rolagem" (E6-04): what stands between the master and the first
 * turn, and the button that starts it. "Começar o combate" is the page's one
 * filled button; while someone has no initiative it is a dashed outline with
 * the reason beside it, and it keeps its focus stop (`aria-disabled`, not
 * `disabled`), so a screen reader hears the reason. A tie does not block
 * the start: the order stays as it is (BeginCombat). "Cancelar combate"
 * ends the combat from SETUP and asks in place first, with the focus on the
 * safe "Voltar".
 */
@Component({
  selector: 'app-initiative-side',
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './initiative-side.html',
  styleUrl: './initiative-side.scss',
})
export class InitiativeSide {
  readonly encounter = input.required<Encounter>();
  readonly info = input<ReadonlyMap<string, CombatantInfo>>(new Map());
  readonly busy = input(false);

  readonly begin = output<void>();
  /** "Cancelar combate", confirmed. */
  readonly cancel = output<void>();

  protected readonly confirming = signal(false);
  private readonly back = viewChild('back', { read: ElementRef<HTMLButtonElement> });

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
    this.ready() ? 'Tudo pronto' : this.missing().length === 1 ? 'Falta uma rolagem' : 'Faltam rolagens',
  );

  constructor() {
    effect(() => this.back()?.nativeElement.focus());
  }

  protected start(): void {
    if (this.ready() && !this.busy()) {
      this.begin.emit();
    }
  }

  protected confirmCancel(): void {
    this.confirming.set(false);
    this.cancel.emit();
  }
}
