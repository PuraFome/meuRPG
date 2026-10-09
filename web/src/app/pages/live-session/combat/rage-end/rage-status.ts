import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { Combatant } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { isRaging } from './rage';

/**
 * What a raging barbarian's card says about the rage: whether it attacked a hostile
 * creature or took damage since its last turn (the two things that keep a rage going,
 * SRD 5.1, Barbarian, Rage), and "Encerrar fúria", the bonus action that ends it.
 * Nothing is drawn for a combatant that is not raging. The master and the owner get
 * the two facts from the server; to anyone else they are false, and the line is not shown.
 */
@Component({
  selector: 'app-rage-status',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule],
  template: `
    @if (raging()) {
      <div class="rage">
        <p class="rage__line" role="status" aria-live="polite">
          <mat-icon aria-hidden="true">local_fire_department</mat-icon>
          <span>
            Atacou um hostil: <b>{{ subject().attackedHostileSinceLastTurn ? 'sim' : 'não' }}</b> ·
            Sofreu dano: <b>{{ subject().tookDamageSinceLastTurn ? 'sim' : 'não' }}</b>
          </span>
        </p>
        @if (canEnd()) {
          <button mat-stroked-button type="button" class="rage__end" [disabled]="busy()" (click)="endRage.emit()">
            Encerrar fúria
          </button>
        }
      </div>
    }
  `,
  styles: `
    :host {
      display: block;
    }

    .rage {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 8px 16px;
    }

    .rage__line {
      display: flex;
      align-items: center;
      gap: 8px;
      margin: 0;
      font-size: 14px;
      line-height: 20px;
    }

    .rage__end {
      --mat-button-outlined-container-height: 44px;
      --mat-button-outlined-label-text-color: var(--mr-ink);
      --mat-button-outlined-outline-color: var(--mr-control-line);
    }
  `,
})
export class RageStatus {
  readonly subject = input.required<Combatant>();
  readonly busy = input(false);
  /** "Encerrar fúria" is offered (the owner's turn, or the master). */
  readonly canEnd = input(true);

  readonly endRage = output<void>();

  protected readonly raging = computed(() => isRaging(this.subject()));
}
