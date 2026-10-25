import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { Combatant } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatantInitial } from '../../../../core/combat/combat-view';
import { moveLine } from '../../../../core/combat/move-line';
import { CombatantToken } from '../../../../shared/combatant-token/combatant-token';
import { EconomyShapes, economyShapes } from '../../../../shared/economy-shapes/economy-shapes';
import type { OwnHitPoints } from '../turn-panel/own-numbers';

/**
 * The turn card of a player character on the master's "Turno" (C8-01, C8-02): "Vez de Pensantus", whose player plays on their own
 * screen, the numbers the master needs (hit points, armor class, movement), the turn's economy as shapes and "Agir por Pensantus", one
 * outlined button. The button is on every player turn: there is no "absent" mode to switch on first. A character at 0 hit points has no
 * button, its card is the death save (C8-03).
 */
@Component({
  selector: 'app-act-as-card',
  imports: [CombatantToken, EconomyShapes, MatButtonModule, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="card" [attr.aria-label]="'Vez de ' + who().label" data-testid="act-as-card">
      <div class="card__head">
        <app-combatant-token [initial]="initial()" [current]="true" [size]="48" />
        <div class="card__who">
          <h2 class="card__title">Vez de {{ who().label }}</h2>
          <p class="card__sub">{{ sub() }}</p>
        </div>
      </div>
      <dl class="tiles">
        <div class="tile">
          <dt>Pontos de vida</dt>
          <dd>
            @if (hitPoints(); as hp) {
              <b>{{ hp.current }}</b> de {{ hp.max }}
            } @else {
              <b>—</b>
            }
          </dd>
        </div>
        <div class="tile">
          <dt>CA</dt>
          <dd>
            <b>{{ who().armorClass ?? '—' }}</b>
          </dd>
        </div>
        <div class="tile">
          <dt>Deslocamento</dt>
          <dd>
            <b>{{ speed() }}</b>
          </dd>
        </div>
      </dl>
      <app-economy-shapes [shapes]="shapes()" />
      <p class="card__wait">
        Esperando {{ who().label }}. Se o jogador não estiver, jogue este turno por ele.
      </p>
      <button mat-stroked-button type="button" class="card__act" [disabled]="busy()" (click)="act.emit()">
        <mat-icon aria-hidden="true">person</mat-icon>Agir por {{ who().label }}
      </button>
    </section>
  `,
  styleUrl: './act-as-card.scss',
})
export class ActAsCard {
  /** The character on turn. */
  readonly who = input.required<Combatant>();
  /** "Mago 3": the class line the page knows. */
  readonly classLine = input('');
  /** The character's hit points from its vitals (the master reads the numbers). */
  readonly hitPoints = input<OwnHitPoints | null>(null);
  readonly busy = input(false);
  /** "Agir por …": the master takes the turn for the character. */
  readonly act = output<void>();

  protected readonly initial = computed(() => combatantInitial(this.who().label));
  protected readonly sub = computed(() =>
    [this.classLine(), 'o jogador joga na tela dele'].filter(Boolean).join(' · '),
  );
  protected readonly line = computed(() => moveLine(this.who(), false));
  protected readonly speed = computed(() => this.line().left);
  protected readonly shapes = computed(() =>
    economyShapes(this.who(), { moveText: this.line().canMove ? this.line().left : null }),
  );
}
