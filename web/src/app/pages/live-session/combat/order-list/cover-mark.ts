import { ChangeDetectionStrategy, Component, ElementRef, afterNextRender, inject, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';

import { CoverDegree } from '../../../../../gen/meurpg/play/v1/combat_pb';

interface Choice {
  readonly value: CoverDegree;
  readonly name: string;
  readonly sub: string;
}

let nextId = 0;

/**
 * "Marcar cobertura" (E9-07, MR-034): the master's cover mark for what the map
 * does not draw (a creature crouched behind an overturned table), a radio group
 * in place, on the combatant's row. The choice applies at once, with no confirm
 * button: "Fechar" only closes. It adds to the map's cover (the larger of the two
 * counts) and clears by itself when the combatant moves. The focus starts on
 * the radio that is marked, and the group's name says whose cover it is.
 */
@Component({
  selector: 'app-cover-mark',
  imports: [MatButtonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="mark">
      <p class="mark__title" [id]="id + '-t'">Cobertura marcada pelo mestre</p>
      <p class="mark__text">
        Vale além da do mapa: a maior das duas conta. Aplica na hora e sai sozinha quando {{ label() }} se mover.
      </p>
      <div class="mark__group" role="radiogroup" [attr.aria-label]="'Cobertura marcada de ' + label()">
        @for (c of choices; track c.value) {
          <label class="mark__item" [class.mark__item--on]="c.value === current()">
            <input
              type="radio"
              class="mr-visually-hidden"
              [name]="id"
              [checked]="c.value === current()"
              (change)="choose($event, c.value)"
            />
            <span class="mark__dot" aria-hidden="true"></span>
            <span class="mark__words">
              <b>{{ c.name }}</b>
              <span class="mark__sub">{{ c.sub }}</span>
            </span>
          </label>
        }
      </div>
      <button mat-stroked-button type="button" class="mark__close" (click)="close.emit()">Fechar</button>
    </div>
  `,
  styleUrl: './cover-mark.scss',
})
export class CoverMark {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  /** The combatant's name. */
  readonly label = input.required<string>();
  readonly current = input<CoverDegree>(CoverDegree.NONE);
  readonly busy = input(false);
  readonly pick = output<CoverDegree>();
  readonly close = output<void>();

  protected readonly id = `cover-${nextId++}`;
  protected readonly choices: readonly Choice[] = [
    { value: CoverDegree.NONE, name: 'Nenhuma', sub: 'Só vale a do mapa' },
    { value: CoverDegree.HALF, name: 'Meia cobertura', sub: '+2 na CA e em Destreza' },
    { value: CoverDegree.THREE_QUARTERS, name: 'Três quartos', sub: '+5 na CA e em Destreza' },
    { value: CoverDegree.TOTAL, name: 'Cobertura total', sub: 'Não pode ser alvo' },
  ];

  /** A choice applies at once. While a call is in flight another is ignored and the radio goes back
   * to what is marked (the control keeps its focus: it is never disabled under the person's hands). */
  protected choose(event: Event, value: CoverDegree): void {
    if (this.busy()) {
      (event.target as HTMLInputElement).checked = value === this.current();
      return;
    }
    this.pick.emit(value);
  }

  constructor() {
    // Opens on the one that is marked.
    afterNextRender(() => {
      const root = this.host.nativeElement;
      (root.querySelector<HTMLInputElement>('input:checked') ?? root.querySelector<HTMLInputElement>('input'))?.focus();
    });
  }
}
