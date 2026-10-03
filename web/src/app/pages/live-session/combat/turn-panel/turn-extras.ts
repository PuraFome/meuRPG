import { Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

/**
 * What goes under "Sua reação" when it is not the player's turn (E6-28, 3rd
 * frame): "Ataque de oportunidade", a text action that opens the attack sheet
 * with the melee attacks only and spends the reaction. With one melee attack it
 * is named plain; with two, each says its weapon. Only drawn when the reaction
 * is free and the character can reach someone with a melee attack.
 */
@Component({
  selector: 'app-turn-reaction',
  imports: [MatButtonModule, MatIconModule],
  template: `
    @if (opportunities().length) {
      <div class="opp">
        @for (o of opportunities(); track o.key) {
          <button
            mat-button
            type="button"
            class="opp__btn"
            [attr.aria-label]="'Ataque de oportunidade com ' + o.name"
            (click)="opportunity.emit(o.key)"
          >
            <mat-icon aria-hidden="true">swords</mat-icon>Ataque de oportunidade{{ opportunities().length > 1 ? ' (' + o.name + ')' : '' }}
          </button>
        }
        <p class="opp__hint">Quando um inimigo sair do seu alcance, ataque com a reação. Só ataques corpo a corpo.</p>
      </div>
    }
  `,
  styles: `
    :host {
      display: contents;
    }

    .opp {
      display: flex;
      flex-direction: column;
      align-items: flex-start;
      gap: 4px;
    }

    // A text button that starts a line pulls its padding back, so its icon
    // lines up with the text edge above.
    .opp__btn {
      margin-left: -12px;
      --mat-button-text-label-text-color: var(--mr-accent-text);
    }

    .opp__hint {
      margin: 0;
      font-size: 14px;
      line-height: 19px;
      color: var(--mr-ink-muted);
    }
  `,
})
export class TurnReaction {
  readonly opportunities = input<readonly { key: string; name: string }[]>([]);
  readonly opportunity = output<string>();
}

/**
 * "Concentrado em Teia" and "Encerrar concentração" (E6-29): a player sees
 * their own concentration on their turn and may end it themselves (the
 * conditions are the master's alone). The spell's name comes with the combat.
 */
@Component({
  selector: 'app-concentration-line',
  imports: [MatButtonModule, MatIconModule],
  template: `
    <div class="conc">
      <p class="conc__text">
        <mat-icon aria-hidden="true">self_improvement</mat-icon>
        <span>Concentrado em <b>{{ spell() }}</b></span>
      </p>
      <button mat-button type="button" class="conc__end" [disabled]="busy()" (click)="end.emit()">Encerrar concentração</button>
    </div>
  `,
  styles: `
    :host {
      display: block;
    }

    /* On a phone the text button goes under the line and pulls its padding back
       so its words start on the text edge; from 768px it sits at the right end. */
    .conc {
      display: flex;
      flex-direction: column;
      align-items: flex-start;
      gap: 0 12px;
    }

    .conc__text {
      display: flex;
      align-items: center;
      gap: 8px;
      margin: 0;
      font-size: 16px;
    }

    .conc__end {
      margin-left: -12px;
      --mat-button-text-label-text-color: var(--mr-accent-text);
    }

    @media (min-width: 768px) {
      .conc {
        flex-direction: row;
        flex-wrap: wrap;
        align-items: center;
        justify-content: space-between;
      }

      .conc__end {
        margin-left: 0;
        margin-right: -12px;
      }
    }
  `,
})
export class ConcentrationLine {
  readonly spell = input('');
  readonly busy = input(false);
  readonly end = output<void>();
}
