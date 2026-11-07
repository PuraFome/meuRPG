import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  computed,
  inject,
  input,
  output,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import {
  type LegendaryCard as LegendaryCardModel,
  type LegendaryRow,
  legendaryFolded,
} from '../../../../core/combat/legendary-card';
import { CountPips } from '../../../../shared/count-pips/count-pips';
import { focusInitial } from '../reaction-queue/offer-answer';

/** What went wrong with the last answer of a card: no connection (the same answer is sent again) or a refusal. */
export interface LegendaryFailure {
  readonly text: string;
  readonly connection: boolean;
}

/**
 * The master's card of one legendary creature's actions: crown and "Ações lendárias", the badge
 * ("Opcional", "Depois"), the creature, what is left of the round and one row per option of its stat block. Folding it
 * ("Agora não") is the screen's own business and sends nothing. The card only draws what the model says and reports the
 * taps: the host (`LegendaryOffers`) sends the calls. Master only; no player component draws it.
 */
@Component({
  selector: 'app-legendary-card',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CountPips, MatButtonModule, MatIconModule],
  templateUrl: './legendary-card.html',
  styleUrls: ['../reaction-queue/offer.scss', './legendary-card.scss'],
})
export class LegendaryCard {
  readonly card = input.required<LegendaryCardModel>();
  /** "Agora não": the card is a line until the next turn. */
  readonly folded = input(false);
  /** The key of the option whose answer is in the air, or `''`. */
  readonly sending = input('');
  readonly failure = input<LegendaryFailure | null>(null);
  /** "Dragão vermelho adulto gastou 2 ações lendárias: Ataque de asas" while the last spend can be undone, else `''`. */
  readonly spentLine = input('');
  /** The log event of that spend: the one "Desfazer última ação" takes back, as it was when the line was drawn. */
  readonly spentId = input('');
  /** Any answer of any card is in the air (a spend, or an attack sheet being opened): no row of any card can be tapped. */
  readonly busy = input(false);
  readonly undoing = input(false);
  /** The key of the move option that waits for the token to be dragged on the map, or `''`. */
  readonly armed = input('');
  /** On a phone the map is another tab: the armed line says where to drag. */
  readonly mapElsewhere = input(false);
  /** The combat has no map: a legendary move needs squares, so a move row is off. */
  readonly theatre = input(false);

  readonly fold = output<void>();
  readonly unfold = output<void>();
  readonly use = output<LegendaryRow>();
  readonly retry = output<void>();
  readonly undo = output<string>();
  readonly cancelMove = output<void>();

  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  protected readonly foldedText = (): string => legendaryFolded(this.card());

  /** The spend line with the creature's label apart, to be set in bold (`['', line]` when it does not start with it). */
  protected readonly spentParts = computed<readonly [string, string]>(() => {
    const line = this.spentLine();
    const title = this.card().title;
    return line.startsWith(title) ? [title, line.slice(title.length)] : ['', line];
  });

  /** "Agora não" and "Abrir": the card redraws as the other state, so the focus goes to the button that undoes it. */
  protected toggleFold(on: boolean): void {
    (on ? this.fold : this.unfold).emit();
    focusInitial(this.host.nativeElement, this.injector);
  }

  /** A move without a map is off: there is no square to walk to. */
  protected reasonOf(row: LegendaryRow): string {
    return (
      row.reason ||
      (row.action === 'move' && this.theatre()
        ? 'Sem mapa: o movimento lendário precisa de um quadrado.'
        : '')
    );
  }

  protected offOf(row: LegendaryRow): boolean {
    return row.disabled || this.reasonOf(row) !== '';
  }

  protected iconOf(row: LegendaryRow): string {
    return row.action === 'attack' ? 'swords' : 'bolt';
  }

  protected labelOf(row: LegendaryRow): string {
    if (this.sending() === row.key) {
      return row.action === 'spend' ? 'Usando…' : 'Abrindo…';
    }
    return this.armed() === row.key ? 'Arraste a ficha' : row.buttonLabel;
  }
}
