import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import {
  HitRiderChoice,
  HitRiderKind,
  type Combatant,
  type HitRiderOffer,
} from '../../../../../gen/meurpg/play/v1/combat_pb';

/** What the monk picks on an offer. */
export interface RiderPick {
  readonly id: string;
  readonly choice: HitRiderChoice;
}

/**
 * The monk's riders after a hit (SRD 5.1, Monk): the Open Hand technique on a Flurry of
 * Blows hit, and Stunning Strike (1 ki point) on a melee hit. Each offer is optional and
 * answered once. The target's save is rolled by the app; the DC is the ki save DC.
 */
@Component({
  selector: 'app-hit-rider-card',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule],
  template: `
    @for (o of offers(); track o.id) {
      <div class="mr-notice rider" role="group" [attr.aria-label]="title(o)" data-testid="hit-rider">
        <mat-icon aria-hidden="true">sports_martial_arts</mat-icon>
        <div class="rider__body">
          <p class="rider__text">
            <strong>{{ title(o) }}</strong>
            em {{ label(o.targetId) }}
          </p>
          <div class="rider__btns">
            @if (isOpenHand(o)) {
              <div class="rider__opt">
                <button
                  mat-stroked-button
                  type="button"
                  [disabled]="busy()"
                  [attr.aria-describedby]="hintId(o, 'prone')"
                  (click)="pick(o, choices.PRONE)"
                >
                  Técnica da Mão Aberta: Derrubar (teste de Destreza)
                </button>
                <p class="rider__hint" [id]="hintId(o, 'prone')">{{ proneHint(o) }}</p>
              </div>
              <div class="rider__opt">
                <button
                  mat-stroked-button
                  type="button"
                  [disabled]="busy()"
                  [attr.aria-describedby]="hintId(o, 'push')"
                  (click)="pick(o, choices.PUSH)"
                >
                  Empurrar até 4,5 m (teste de Força)
                </button>
                <p class="rider__hint" [id]="hintId(o, 'push')">{{ pushHint(o) }}</p>
              </div>
              <div class="rider__opt">
                <button
                  mat-stroked-button
                  type="button"
                  [disabled]="busy()"
                  [attr.aria-describedby]="hintId(o, 'none')"
                  (click)="pick(o, choices.NO_REACTIONS)"
                >
                  Sem reações até o fim do seu próximo turno
                </button>
                <p class="rider__hint" [id]="hintId(o, 'none')">{{ noReactionsHint }}</p>
              </div>
            } @else {
              <div class="rider__opt">
                <button
                  mat-stroked-button
                  type="button"
                  [disabled]="busy() || o.kiLeft < 1"
                  [attr.aria-describedby]="hintId(o, 'stun')"
                  (click)="pick(o, choices.STUN)"
                >
                  Golpe Atordoante (1 ki)
                </button>
                <p class="rider__hint" [id]="hintId(o, 'stun')">{{ stunHint(o) }}</p>
              </div>
            }
            <button mat-button type="button" [disabled]="busy()" (click)="pick(o, choices.DECLINE)">Dispensar</button>
          </div>
        </div>
      </div>
    }
  `,
  styles: `
    :host {
      display: contents;
    }

    .rider {
      align-items: flex-start;
    }

    .rider__body {
      display: flex;
      flex: 1;
      flex-direction: column;
      gap: 12px;
      min-width: 0;
    }

    .rider__text {
      margin: 0;
    }

    .rider__opt {
      display: flex;
      flex-direction: column;
      gap: 4px;
    }

    .rider__hint {
      color: var(--mr-ink-muted);
      font-size: 0.875rem;
      margin: 0;
    }

    .rider__btns {
      display: flex;
      flex-direction: column;
      gap: 10px;
    }
  `,
})
export class HitRiderCard {
  readonly offers = input<readonly HitRiderOffer[]>([]);
  readonly combatants = input<readonly Combatant[]>([]);
  readonly busy = input(false);

  readonly answer = output<RiderPick>();

  protected readonly choices = HitRiderChoice;
  private readonly labels = computed(() => new Map(this.combatants().map((c) => [c.id, c.label])));

  protected isOpenHand(o: HitRiderOffer): boolean {
    return o.kind === HitRiderKind.OPEN_HAND;
  }

  protected title(o: HitRiderOffer): string {
    return this.isOpenHand(o) ? 'Técnica da Mão Aberta' : 'Golpe Atordoante';
  }

  protected readonly noReactionsHint =
    'Sem teste de resistência: o alvo não pode usar reações até o fim do seu próximo turno.';

  /** A unique id for the hint a button is described by. */
  protected hintId(o: HitRiderOffer, what: string): string {
    return `rider-${o.id}-${what}`;
  }

  /** "Teste de Destreza CD 14." The DC is the monk's own ki save DC (never an NPC's number, RN-20); left out when the server sends none. */
  private saveText(ability: string, o: HitRiderOffer): string {
    return o.saveDc > 0 ? `Teste de ${ability} CD ${o.saveDc}.` : `Teste de ${ability}.`;
  }

  protected proneHint(o: HitRiderOffer): string {
    return `${this.saveText('Destreza', o)} Se falhar, fica derrubado.`;
  }

  protected pushHint(o: HitRiderOffer): string {
    return `${this.saveText('Força', o)} Se falhar, é empurrado até 4,5 m.`;
  }

  /** What Stunning Strike costs and does; with no ki left, the reason it is off. */
  protected stunHint(o: HitRiderOffer): string {
    const cost =
      o.kiLeft < 1
        ? 'Sem ki: você não tem pontos de ki para gastar.'
        : `Gasta 1 ponto de ki (restam ${o.kiLeft}, ficam ${o.kiLeft - 1}).`;
    return `${cost} ${this.saveText('Constituição', o)} Se falhar, fica atordoado até o fim do seu próximo turno.`;
  }

  protected label(id: string): string {
    return this.labels().get(id) ?? '';
  }

  protected pick(o: HitRiderOffer, choice: HitRiderChoice): void {
    this.answer.emit({ id: o.id, choice });
  }
}
