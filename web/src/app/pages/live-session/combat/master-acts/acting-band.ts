import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { combatantInitial } from '../../../../core/combat/combat-view';
import { CombatantToken } from '../../../../shared/combatant-token/combatant-token';

/**
 * The band at the top of the master's rail while he acts for a character (C8-01, C8-02): "Você está agindo por Pensantus", the
 * sentence that what he does there is the character's, and "Voltar ao mestre", which gives the rail back to the master without
 * ending the turn.
 */
@Component({
  selector: 'app-acting-band',
  imports: [CombatantToken, MatButtonModule, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="band" aria-label="Agindo por um personagem" data-testid="acting-band">
      <div class="band__head">
        <app-combatant-token [initial]="initial()" [current]="true" [size]="44" />
        <div class="band__text" role="status">
          <h2 class="band__title">Você está agindo por {{ name() }}</h2>
          <p class="band__sub">O que você fizer aqui é dele, como se ele fizesse.</p>
        </div>
      </div>
      <button mat-stroked-button type="button" class="band__back" (click)="back.emit()">
        <mat-icon aria-hidden="true">chevron_left</mat-icon>Voltar ao mestre
      </button>
    </section>
  `,
  styleUrl: './acting-band.scss',
})
export class ActingBand {
  readonly name = input.required<string>();
  /** "Voltar ao mestre": the rail goes back to the master; the turn goes on. */
  readonly back = output<void>();

  protected readonly initial = computed(() => combatantInitial(this.name()));
}
