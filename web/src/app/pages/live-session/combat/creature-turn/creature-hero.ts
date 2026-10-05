import { Component, ElementRef, computed, effect, input, output, viewChild } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { Combatant, Encounter } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { creatureTurnTitle, endLabel as endLabelOf, groupFeminine, groupName } from '../../../../core/combat/creature-names';
import { roundLabel, turnBanner } from '../../../../core/combat/combat-view';
import { type MineTab } from '../../../../core/combat/mine';
import { tieNumbers } from '../../../../core/format/text';
import { mediaQuery } from '../../../../shared/map-view/media-query';
import { EndPart } from '../joint-turn/end-part';
import { EndTurn } from '../turn-panel/end-turn';

/**
 * The card on top of the page of a creature's turn (MR-037, E9-12 states 3 and 6): "Vez dos seus Lobos atrozes",
 * "Depois de vocês: Nanquim" and, from 1024px, the end button (on a phone it is in the pinned bar, with the tabs).
 * It is a live region, so a turn change is heard once, and the title takes the focus when the turn arrives.
 * Off its turn it says where the creatures stand ("esperam a vez deles", "já agiram"). Like the character's card, it
 * has a 2px accent frame while it acts. The end button is "Encerrar a parte dos Lobos" for a group (a joint
 * turn: it ends the part of each member who still acts) and "Encerrar a vez do Nanquim" for one creature.
 */
@Component({
  selector: 'app-creature-hero',
  imports: [EndPart, EndTurn, MatIconModule],
  template: `
    <section class="hero" [class.hero--turn]="acting()" aria-label="Vez das suas criaturas">
      <span class="mr-label hero__kicker">{{ round() }} · {{ encounter().name }}</span>
      <div class="hero__banner" role="status" aria-live="polite">
        <h2 #hero class="hero__title" tabindex="-1">{{ title() }}</h2>
      </div>
      @if (after(); as a) {
        <p class="hero__next">{{ a }}</p>
      }
      @if (note()) {
        <div class="mr-notice mr-notice--warning hero__notice" role="status">
          <mat-icon aria-hidden="true">hourglass_top</mat-icon>
          <p>{{ note() }}</p>
        </div>
      }
      @if (desktop() && acting()) {
        <div class="hero__end">
          @if (group()) {
            <app-end-part
              [label]="endLabel()"
              [heading]="endLabel() + '?'"
              [confirmLabel]="endLabel()"
              warning="Não dá para reabrir esta parte depois."
              [left]="left()"
              [busy]="busy()"
              (endPart)="endPart.emit()"
            />
          } @else {
            <app-end-turn [own]="tab().members[0]" [label]="endLabel()" [busy]="busy()" [waiting]="waiting()" (endTurn)="endPart.emit()" />
          }
        </div>
      }
    </section>
  `,
  styleUrl: './creature-hero.scss',
})
export class CreatureHero {
  readonly encounter = input.required<Encounter>();
  readonly tab = input.required<MineTab>();
  readonly busy = input(false);
  /** The turn waits for an answer ("Esperando a reação do mestre"): the end button is off. */
  readonly waiting = input('');
  /** What the turn waits for, and what the last move said: lines under the title. */
  readonly note = input('');

  /** "Encerrar a parte dos Lobos" / "Encerrar a vez do Nanquim": the whole tab ends. */
  readonly endPart = output<void>();

  private readonly heading = viewChild<ElementRef<HTMLElement>>('hero');
  protected readonly desktop = mediaQuery('(min-width: 1024px)');

  protected readonly acting = computed(() => this.tab().state === 'turn');
  protected readonly group = computed(() => this.tab().members.length > 1);
  protected readonly round = computed(() => tieNumbers(roundLabel(this.encounter().round)));
  protected readonly name = computed(() => groupName(this.tab().members));
  protected readonly title = computed(() => {
    const t = this.tab();
    if (t.state === 'turn') {
      return creatureTurnTitle(this.encounter(), t.members, true);
    }
    const several = t.members.length > 1;
    const fem = groupFeminine(t.members);
    const who = several ? `${fem ? 'As suas' : 'Os seus'} ${this.name()}` : `${fem ? 'A sua' : 'O seu'} ${this.name()}`;
    return t.state === 'done' ? `${who} já ${several ? 'agiram' : 'agiu'}` : `${who} ${several ? 'esperam' : 'espera'} a vez ${several ? 'deles' : 'dele'}`;
  });
  /** "Depois de vocês: Nanquim": who plays after this turn. */
  protected readonly after = computed(() => {
    if (!this.acting()) {
      return '';
    }
    const after = turnBanner(this.encounter()).after;
    return after ? `${this.group() ? 'Depois de vocês' : 'Depois de você'}: ${after.name}` : '';
  });
  protected readonly endLabel = computed(() => endLabelOf(this.tab().members));
  /** What a part still has, for the question before ending it. */
  protected readonly left = computed(() => (this.tab().acting.some((m: Combatant) => !m.actionUsed) ? 'ações' : ''));

  constructor() {
    // Focus moves to the title when the turn arrives, once, so the next Tab reaches the blocks.
    let was = false;
    effect(() => {
      const now = this.acting();
      if (now && !was) {
        queueMicrotask(() => this.heading()?.nativeElement.focus({ preventScroll: true }));
      }
      was = now;
    });
  }
}
