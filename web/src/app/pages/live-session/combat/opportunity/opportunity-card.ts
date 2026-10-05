import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  effect,
  inject,
  input,
  output,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { Encounter, OpportunityOffer } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { metersFixed } from '../../../../core/units';
import {
  type ReactorAttack,
  attackLabel,
  masterAsk,
  masterNews,
  offersToAnswer,
  reactorIsMasters,
} from '../../../../core/combat/opportunity';
import { article } from '../../../../core/combat/combat-log';
import { ofThe } from '../../../../core/combat/move-plan';
import type { CombatantInfo } from '../combat-info';

/** The master's answer to an NPC's offer: "Atacar com X", or "Não atacar". */
export interface MasterAnswer {
  readonly offer: OpportunityOffer;
  readonly attack: ReactorAttack | null;
}

/**
 * The master's opportunity prompts, on the combat screen (E9-13). Two kinds:
 *
 * - **An NPC's reactor** (the master answers): "O Toren saiu do alcance do Goblin
 *   2. Goblin 2 ataca o Toren?", the attack with its numbers and "gasta a
 *   reação dele", and "Não atacar" (the focus lands there: the safe answer) with
 *   "Atacar com <arma>", the same width. The turn of the mover waits.
 * - **A player's reactor**: "Esperando a reação do Caio (Toren)", with "Seguir
 *   sem esperar", which takes that reaction away from the player when they do not
 *   answer (offline). The prompt stays on the player's screen until they answer
 *   or the master skips it.
 *
 * It lists what the server sent the master (`Encounter.opportunity_offers`):
 * the browser works out neither who provokes whom nor who may answer.
 */
@Component({
  selector: 'app-opportunity-card',
  imports: [MatButtonModule, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @for (o of offers(); track o.id) {
      <section class="op" role="group" [attr.aria-label]="'Ataque de oportunidade de ' + o.reactorLabel">
        <h2 class="op__title"><mat-icon aria-hidden="true">swords</mat-icon>Ataque de oportunidade</h2>
        <p class="op__news">{{ news(o) }}</p>
        @if (mine(o)) {
          <p class="op__ask"><b>{{ ask(o) }}</b></p>
          @if (attacks(o)[0]; as first) {
            <p class="op__sub">{{ first.detail }} · gasta a reação dele</p>
          }
          <div class="op__pair">
            <button mat-stroked-button type="button" class="op__btn" data-safe [disabled]="busy()" (click)="answer.emit({ offer: o, attack: null })">
              Não atacar
            </button>
            @for (a of attacks(o); track a.key) {
              <button mat-flat-button type="button" class="op__btn" [disabled]="busy()" (click)="answer.emit({ offer: o, attack: a })">
                {{ label(a) }}
              </button>
            }
          </div>
          <p class="op__note">
            <mat-icon aria-hidden="true">info</mat-icon>
            O turno {{ ofMover(o) }} espera a sua resposta. {{ walked(o) }}
          </p>
          <p class="op__note op__note--plain">
            O ataque de oportunidade ignora o alcance. Se ele levar {{ theMover(o) }} a 0 PV, volta ao último quadrado que ainda
            estava no alcance.
          </p>
        } @else {
          <p class="op__ask"><b>Esperando a reação {{ waitingFor(o) }}</b></p>
          <div class="op__pair">
            <button mat-stroked-button type="button" class="op__btn" [disabled]="busy()" (click)="skip.emit(o)">
              Seguir sem esperar
            </button>
          </div>
          <p class="op__note op__note--plain">
            Se você seguir sem esperar, {{ theReactor(o) }} perde essa reação. O aviso fica na tela do jogador até ele responder ou
            você pular.
          </p>
        }
      </section>
    }
  `,
  styleUrl: './opportunity-card.scss',
})
export class OpportunityCard {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);
  readonly encounter = input.required<Encounter>();
  /** Each NPC reactor's attacks with their numbers, by offer id. */
  readonly attacksByOffer = input<ReadonlyMap<string, readonly ReactorAttack[]>>(new Map());
  readonly info = input<ReadonlyMap<string, CombatantInfo>>(new Map());
  readonly busy = input(false);

  readonly answer = output<MasterAnswer>();
  readonly skip = output<OpportunityOffer>();

  protected readonly offers = computed(() => offersToAnswer(this.encounter()));

  constructor() {
    // A prompt that appears puts the focus on the safe answer ("Não atacar").
    let seen = '';
    effect(() => {
      const ids = this.offers().map((o) => o.id).join(',');
      if (ids !== '' && ids !== seen) {
        afterNextRender(() => this.host.nativeElement.querySelector<HTMLButtonElement>('[data-safe]')?.focus(), {
          injector: this.injector,
        });
      }
      seen = ids;
    });
  }

  protected mine(o: OpportunityOffer): boolean {
    return reactorIsMasters(this.encounter(), o);
  }

  protected news(o: OpportunityOffer): string {
    return masterNews(o);
  }

  protected ask(o: OpportunityOffer): string {
    return masterAsk(o);
  }

  protected attacks(o: OpportunityOffer): readonly ReactorAttack[] {
    return this.attacksByOffer().get(o.id) ?? [];
  }

  protected label(a: ReactorAttack): string {
    return attackLabel(a);
  }

  protected ofMover(o: OpportunityOffer): string {
    return ofThe([o.moverLabel]);
  }

  protected theMover(o: OpportunityOffer): string {
    return `${article(o.moverLabel)} ${o.moverLabel}`;
  }

  protected theReactor(o: OpportunityOffer): string {
    return `${article(o.reactorLabel)} ${o.reactorLabel}`;
  }

  /** "Ele já andou 4,5 m: o movimento valeu." */
  protected walked(o: OpportunityOffer): string {
    const mover = this.encounter().combatants.find((c) => c.id === o.moverId);
    const used = mover?.movementUsedDft ?? 0;
    return used > 0 ? `Já andou ${metersFixed(used / 10)}: o movimento valeu.` : 'O movimento valeu.';
  }

  /** "do Caio (Toren)": the player's name when the roster has it. */
  protected waitingFor(o: OpportunityOffer): string {
    const reactor = this.encounter().combatants.find((c) => c.id === o.reactorId);
    const name = reactor ? (this.info().get(reactor.characterId)?.playerName ?? '') : '';
    return name ? `${ofThe([name])} (${o.reactorLabel})` : `do jogador de ${o.reactorLabel}`;
  }
}
