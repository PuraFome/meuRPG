import { ChangeDetectionStrategy, Component, computed, input, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { effectivePreference } from '../../../../core/campaigns/dice-labels';
import type { HintTry } from '../../../../core/puzzles/puzzle-play';
import type { HintDie } from '../../../../core/puzzles/puzzles-client';
import { RollPicker } from '../../combat/roll-picker/roll-picker';

/**
 * "Tentar uma dica · Investigação" on the player's phone (MR-038, RN-27, RN-18, E10-12 states 9 and 9b): a skill check the master set wins
 * the player the next hint, for them alone. The button names the skill and **never the DC** (the DC is the master's: the server does not
 * send it). The d20 is rolled the way the table rolls (RN-18): in the app, or the face of a real die typed (1 to 20, without the bonus: the
 * server adds the character's own). A forced mode shows only its way; "cada jogador escolhe" starts on the player's own choice and keeps the
 * other as a link. A pass says "Você conseguiu." and that the hint is theirs; a fail says "Não deu desta vez." and never how far it was.
 * The button is there only while `canTry` (the server's answer says when the player may try for the next hint).
 */
@Component({
  selector: 'app-hint-try',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule, RollPicker],
  template: `
    @if (result(); as r) {
      @if (r.passed) {
        <div class="mr-notice mr-notice--success" role="status">
          <mat-icon aria-hidden="true">check</mat-icon>
          <p><strong>Você conseguiu.</strong> Esta dica é só sua; se quiser, conte aos outros.@if (total(r)) { <span class="roll"> Seu total: {{ total(r) }}.</span> }</p>
        </div>
      } @else {
        <div class="mr-notice mr-notice--neutral" role="status">
          <mat-icon aria-hidden="true">close</mat-icon>
          <p>@if (total(r)) {<span class="roll">Você tirou {{ total(r) }}. </span>}<strong>Não deu desta vez.</strong> Outro jogador pode tentar, ou o mestre solta uma dica.</p>
        </div>
      }
    }
    @if (canTry()) {
      @if (typing()) {
        <app-roll-picker
          [canApp]="canApp()"
          [canType]="true"
          [preferApp]="false"
          [label]="'O d20 que você rolou'"
          [hint]="'Teste de ' + skill() + '. Role o seu d20 e digite o dado, sem o bônus.'"
          totalNote="O seu d20"
          [busy]="busy()"
          [outlined]="true"
          [compact]="true"
          [hideTotal]="true"
          [(typing)]="typing"
          (app)="app()"
          (typed)="onTyped($event)"
        />
      } @else {
        <button matButton="outlined" type="button" class="go" [disabled]="busy()" disabledInteractive (click)="go()">
          <mat-icon aria-hidden="true">casino</mat-icon>Tentar uma dica · {{ skill() }}
        </button>
        <p class="help">{{ help() }}</p>
        @if (offerType()) {
          <button matButton type="button" class="link" (click)="typing.set(true)">Digitar o resultado</button>
        }
      }
    }
  `,
  styles: `
    :host {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-3);
    }

    :host:empty {
      display: none;
    }

    .go {
      min-height: 48px;
      width: 100%;
    }

    .help {
      margin: 0;
      font-size: 14px;
      line-height: 20px;
      color: var(--mr-ink-muted);
    }

    .link {
      align-self: flex-start;
      margin-inline-start: -12px;
      min-height: 44px;
      color: var(--mr-accent-text);
    }

    .roll {
      color: var(--mr-ink-muted);
    }
  `,
})
export class HintTryControl {
  /** The skill's name in Portuguese ("Investigação"). Never the DC. */
  readonly skill = input.required<string>();
  /** The server says the player may try now. */
  readonly canTry = input(false);
  readonly busy = input(false);
  readonly diceMode = input<DiceMode>(DiceMode.PLAYERS_CHOOSE);
  readonly dicePreference = input<DicePreference>(DicePreference.APP);
  /** The player's last try, until it changes. */
  readonly result = input<HintTry | null>(null);

  /** A try with the d20 rolled in the app. */
  readonly rolled = output<HintDie>();
  /** A try with the face of a real die. */
  readonly typed = output<HintDie>();

  protected readonly typing = signal(false);
  protected readonly canApp = computed(() => this.diceMode() !== DiceMode.PHYSICAL);
  private readonly canType = computed(() => this.diceMode() !== DiceMode.APP);
  private readonly prefersApp = computed(() => effectivePreference(this.diceMode(), this.dicePreference()) === DicePreference.APP);
  /** "Digitar o resultado" as a link: only when the table lets the player choose and they prefer the app. */
  protected readonly offerType = computed(() => this.canApp() && this.canType() && this.prefersApp());
  protected readonly help = computed(() => {
    if (!this.canType()) {
      return 'A rolagem é no app.';
    }
    if (!this.canApp()) {
      return 'A mesa usa dados físicos: você digita o resultado.';
    }
    return 'A rolagem é no app. Com dado físico, você digita o resultado.';
  });

  protected go(): void {
    if (this.canApp() && (!this.canType() || this.prefersApp())) {
      this.rolled.emit({ inApp: true });
    } else {
      this.typing.set(true);
    }
  }

  protected onTyped(face: number): void {
    this.typing.set(false);
    this.typed.emit({ face });
  }

  protected app(): void {
    this.typing.set(false);
    this.rolled.emit({ inApp: true });
  }

  protected total(r: HintTry): number {
    return r.roll?.total ?? 0;
  }
}
