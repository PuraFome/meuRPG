import { ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { Nudger } from '../../../core/combat/nudger';

/**
 * The master's "Chamar a atenção" next to a player's character (an MSN-style nudge): the player's screen vibrates, chimes
 * and shakes, and says "O mestre está chamando você.". It is the master's alone and only for a character with a player, so
 * the host shows it only there. After a tap it says "Chamado." in a polite live region and stays off for the server's wait
 * (`Nudger`); a refusal is written in the same place. `compact` is the icon-only form for a crowded row. Both are 44 px.
 */
@Component({
  selector: 'app-nudge-button',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule],
  template: `
    @if (compact()) {
      <button
        mat-icon-button
        type="button"
        class="nudge"
        [attr.aria-label]="'Chamar a atenção de ' + name()"
        [disabled]="cooling()"
        (click)="call()"
      >
        <mat-icon aria-hidden="true">notifications_active</mat-icon>
      </button>
    } @else {
      <button
        mat-stroked-button
        type="button"
        class="nudge"
        [attr.aria-label]="'Chamar a atenção de ' + name()"
        [disabled]="cooling()"
        (click)="call()"
      >
        <mat-icon aria-hidden="true">notifications_active</mat-icon>Chamar a atenção
      </button>
    }
    <span class="status" role="status" aria-live="polite">{{ said() }}</span>
  `,
  styles: `
    :host {
      display: inline-flex;
      flex-direction: column;
      align-items: flex-start;
    }

    .nudge {
      min-width: 44px;
      min-height: 44px;
    }

    .status {
      font-size: var(--mr-font-14);
      line-height: 20px;
    }

    .status:empty {
      display: none;
    }
  `,
})
export class NudgeButton {
  private readonly nudger = inject(Nudger);

  readonly campaignId = input.required<string>();
  readonly characterId = input.required<string>();
  /** The character's name, for the accessible name. */
  readonly name = input.required<string>();
  readonly compact = input(false);

  protected readonly cooling = computed(() => this.nudger.cooling().has(this.characterId()));
  protected readonly said = computed(() =>
    this.nudger.messageFor() === this.characterId() ? this.nudger.message() : '',
  );

  protected call(): void {
    void this.nudger.call(this.campaignId(), this.characterId());
  }
}
