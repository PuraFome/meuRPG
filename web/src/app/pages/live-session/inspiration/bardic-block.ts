import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  signal,
  untracked,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { DiceMode } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import type { GetOutsideInspirationResponse } from '../../../../gen/meurpg/play/v1/resources_pb';
import { ActionKey } from '../../../core/connect/idempotency';
import { type InspirationRoll, ResourceClient } from '../../../core/resources/resources-client';
import { classResourceErrorMessage } from '../../../core/resources/resources-errors';
import { InspirationOffer } from './inspiration-offer';

const SECONDS_PER_MINUTE = 60;

/** "5 min", "1 min": the game time a die has left, rounded up to the minute. */
export function minutesText(seconds: number): string {
  const min = Math.max(1, Math.ceil(seconds / SECONDS_PER_MINUTE));
  return `${min} min`;
}

/**
 * "Inspiração de Bardo" on the session panel, outside a combat (SRD 5.1, Bard): the bard sees the uses left and gives the
 * die to a character of the party ("Dar a Tavo"); the character that holds a die reads "Você tem uma Inspiração de Bardo
 * (d8) de Orla" with the game time it has left, and answers the rolls that wait for the die. The master reads the dice
 * held. In a combat the bard gives from the combat's own sheet. It reads again when `tick` moves (the table's hint that
 * the game time or an effect moved) and after each of its own calls.
 */
@Component({
  selector: 'app-bardic-block',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [InspirationOffer, MatButtonModule, MatIconModule],
  template: `
    @if (shown()) {
      <section class="block" aria-labelledby="bardic-t">
        <h2 id="bardic-t" class="t"><mat-icon aria-hidden="true">music_note</mat-icon>Inspiração de Bardo</h2>
        @if (error()) {
          <div class="mr-notice mr-notice--danger" role="alert">
            <mat-icon aria-hidden="true">error</mat-icon>
            <p>{{ error() }}</p>
          </div>
        }
        @if (state(); as s) {
          @if (s.mine; as die) {
            <p class="mine" role="status">
              Você tem uma Inspiração de Bardo (d{{ die.sides }}){{ die.fromName ? ' de ' + die.fromName : '' }}, por mais
              {{ minutes(die.secondsLeft) }} de jogo. Você pode somá-la a uma prova de habilidade, jogada de ataque ou salvaguarda; ao rolar, o app pergunta.
            </p>
          }
          @for (offer of s.offers; track offer.holdId) {
            <app-inspiration-offer
              [offer]="offer"
              [canApp]="canApp()"
              [canType]="canType()"
              [busy]="busy()"
              (use)="answer(offer.holdId, true, $event)"
              (keep)="answer(offer.holdId, false, null)"
            />
          }
          @if (answered(); as text) {
            <p class="mine" role="status">{{ text }}</p>
          }
          @if (s.isBard && !s.combatOpen) {
            <p class="uses">
              Usos: <b>{{ s.usesLeft }}</b> de {{ s.usesMax }} · dado d{{ s.sides }} · ação bônus
            </p>
            <ul class="targets">
              @for (t of s.targets; track t.characterId) {
                <li>
                  <button
                    matButton="outlined"
                    type="button"
                    [disabled]="busy() || s.usesLeft === 0 || t.disabledReasonPt !== ''"
                    disabledInteractive
                    (click)="give(t.characterId, t.name)"
                  >
                    Dar a {{ t.name }}
                  </button>
                  @if (t.disabledReasonPt) {
                    <span class="small">{{ t.disabledReasonPt }}</span>
                  }
                </li>
              } @empty {
                <li class="small">Não há ninguém para inspirar.</li>
              }
            </ul>
          }
          @if (s.held.length > 0) {
            <ul class="held">
              @for (d of s.held; track d.characterId) {
                <li>{{ d.characterName }}: d{{ d.sides }}{{ d.fromName ? ' de ' + d.fromName : '' }}, {{ minutes(d.secondsLeft) }} de jogo</li>
              }
            </ul>
          }
        }
      </section>
    }
  `,
  styles: `
    .block { display: grid; gap: 12px; padding: 16px; border: 1px solid var(--mr-border, #ccc); border-radius: 12px; margin-block: 12px; }
    .t { display: flex; align-items: center; gap: 8px; margin: 0; font-size: 1.05rem; }
    .mine, .uses { margin: 0; }
    .targets, .held { display: grid; gap: 8px; margin: 0; padding: 0; list-style: none; }
    .targets li { display: grid; gap: 4px; }
    .small { font-size: 0.875rem; color: var(--mr-text-muted, inherit); }
  `,
})
export class BardicBlock {
  private readonly api = inject(ResourceClient);

  readonly campaignId = input.required<string>();
  readonly diceMode = input<DiceMode>(DiceMode.PLAYERS_CHOOSE);
  /** Moves when the table's game time or effects move: read the state again. */
  readonly tick = input(0);

  protected readonly state = signal<GetOutsideInspirationResponse | null>(null);
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  protected readonly answered = signal('');
  protected readonly minutes = minutesText;
  protected readonly canApp = computed(() => this.diceMode() !== DiceMode.PHYSICAL);
  protected readonly canType = computed(() => this.diceMode() !== DiceMode.APP);
  protected readonly shown = computed(() => {
    const s = this.state();
    return (
      !!s &&
      (!!s.mine ||
        s.offers.length > 0 ||
        (s.isBard && !s.combatOpen) ||
        s.held.length > 0 ||
        this.answered() !== '')
    );
  });
  private readonly keys = new ActionKey();

  constructor() {
    effect(() => {
      this.tick();
      this.campaignId();
      untracked(() => void this.load());
    });
  }

  private async load(): Promise<void> {
    try {
      this.state.set(await this.api.outsideInspiration(this.campaignId()));
    } catch {
      // A hint that did not come back: the block keeps what it shows and reads again on the next one.
    }
  }

  protected async give(characterId: string, name: string): Promise<void> {
    if (this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      await this.api.giveBardicInspirationOutside(
        this.campaignId(),
        characterId,
        this.keys.keyFor({ give: characterId }),
      );
      this.keys.renew();
      this.answered.set(`Você inspirou ${name}.`);
      await this.load();
    } catch (err) {
      this.error.set(classResourceErrorMessage(err, 'dar a Inspiração de Bardo'));
    } finally {
      this.busy.set(false);
    }
  }

  protected async answer(
    holdId: string,
    use: boolean,
    roll: InspirationRoll | null,
  ): Promise<void> {
    if (this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      const res = await this.api.answerOutsideInspiration(
        this.campaignId(),
        holdId,
        use,
        roll,
        this.keys.keyFor({ holdId, use, roll }),
      );
      this.keys.renew();
      const total =
        res.result.case === 'sceneCheck'
          ? res.result.value.roll?.roll?.total
          : res.result.case === 'groupCheck'
            ? undefined
            : undefined;
      this.answered.set(
        total === undefined
          ? use
            ? 'Dado somado. O mestre vê o resultado.'
            : 'Dado guardado. O mestre vê o resultado.'
          : `${use ? 'Dado somado' : 'Dado guardado'}. Seu total: ${total}.`,
      );
      await this.load();
    } catch (err) {
      this.error.set(classResourceErrorMessage(err, 'responder sobre a Inspiração de Bardo'));
      await this.load();
    } finally {
      this.busy.set(false);
    }
  }
}
