import { ChangeDetectionStrategy, Component, ElementRef, Injector, afterNextRender, computed, effect, inject, input, output, signal, untracked, viewChild } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { MapPoint } from '../../../../../gen/meurpg/maps/v1/maps_pb';
import { foundLine, summaryLine, treasureSub } from '../../../../core/traps/treasure-text';
import { PairFoot } from '../../../../shared/pair-foot/pair-foot';
import { type PickRow, PersonPick } from '../../../../shared/person-pick/person-pick';

/** The master's answer to "Marcar como encontrado": who found it. */
export interface FoundChoice {
  readonly point: MapPoint;
  readonly characterIds: readonly string[];
}

/**
 * One treasure on the master's "Tesouros do mapa" (E9-09 1 to 3, MR-041): its name and value, what is inside
 * (the master's alone until it is found), and:
 * - **hidden:** "Marcar como encontrado" opens a form **in place** (no dialog): "Quem encontrou", nobody
 *   checked, at least one required, the line saying what goes into the session's summary, the one filled
 *   button and "Voltar". The focus goes to the first box; Esc or "Voltar" hands it back to the button;
 * - **found:** "Encontrado" and "Encontrado por Brisa às 21:40"; "Desmarcar" asks in place (an alert dialog,
 *   the focus on "Voltar", two buttons of the same size);
 * - **converted into XP:** "Convertido em XP", the lock and the way to undo it (the XP, on the campaign's page):
 *   there is no "Desmarcar".
 * Presentational: the panel makes the calls.
 */
@Component({
  selector: 'app-treasure-card',
  imports: [MatButtonModule, MatIconModule, PairFoot, PersonPick],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './treasure-card.html',
  styleUrl: './treasure-card.scss',
})
export class TreasureCard {
  private readonly injector = inject(Injector);

  readonly point = input.required<MapPoint>();
  /** The living player characters, with their class and player, for "Quem encontrou". */
  readonly people = input<readonly PickRow[]>([]);
  readonly busy = input(false);
  readonly error = input('');

  readonly mark = output<FoundChoice>();
  readonly unmark = output<MapPoint>();

  protected readonly mode = signal<'view' | 'marking' | 'unmarking'>('view');
  protected readonly picked = signal<ReadonlySet<string>>(new Set());
  private readonly opener = viewChild('opener', { read: ElementRef<HTMLButtonElement> });
  private readonly undo = viewChild('undo', { read: ElementRef<HTMLButtonElement> });
  private readonly form = viewChild<ElementRef<HTMLElement>>('form');
  private readonly ask = viewChild<ElementRef<HTMLElement>>('ask');

  protected readonly found = computed(() => this.point().treasureFoundAt !== undefined);
  protected readonly converted = computed(() => this.point().treasureConverted);
  protected readonly sub = computed(() => treasureSub(this.point()));
  protected readonly line = computed(() => foundLine(this.point()));
  protected readonly names = computed(() => this.people().filter((p) => this.picked().has(p.id)).map((p) => p.name));
  protected readonly summary = computed(() => summaryLine(this.names(), this.point().treasureValuePo));

  protected startMarking(): void {
    this.picked.set(new Set());
    this.mode.set('marking');
    // The button that opened the form is gone: the whole form into view, the focus on its first box.
    afterNextRender(
      () => {
        const form = this.form()?.nativeElement;
        form?.scrollIntoView({ block: 'nearest' });
        form?.querySelector<HTMLInputElement>('input[type=checkbox]:not(:disabled)')?.focus({ preventScroll: true });
      },
      { injector: this.injector },
    );
  }

  protected cancelMarking(): void {
    this.mode.set('view');
    afterNextRender(() => this.opener()?.nativeElement.focus(), { injector: this.injector });
  }

  protected confirmMarking(): void {
    if (this.picked().size > 0 && !this.busy()) {
      this.mark.emit({ point: this.point(), characterIds: this.people().filter((p) => this.picked().has(p.id)).map((p) => p.id) });
    }
  }

  protected startUnmark(): void {
    this.mode.set('unmarking');
    afterNextRender(
      () => {
        this.ask()?.nativeElement.scrollIntoView({ block: 'nearest' });
        this.ask()?.nativeElement.querySelector<HTMLButtonElement>('[data-initial-focus]')?.focus({ preventScroll: true });
      },
      { injector: this.injector },
    );
  }

  protected cancelUnmark(): void {
    this.mode.set('view');
    afterNextRender(() => this.undo()?.nativeElement.focus(), { injector: this.injector });
  }

  constructor() {
    // The answer came back (found, or unmarked): the card is a plain card again.
    effect(() => {
      this.found();
      untracked(() => this.mode.set('view'));
    });
  }
}
