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
  viewChild,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { Combatant } from '../../../../../gen/meurpg/play/v1/combat_pb';

/**
 * "A sua fúria vai acabar?" (SRD 5.1, Barbarian, Rage): the question a raging
 * barbarian's player is asked when the turn ends with no attack on a hostile creature
 * and no damage taken. The master is asked too ("A fúria de Toren vai acabar?"). "Voltar e atacar"
 * keeps the turn (so does Escape); "Deixar a fúria acabar" ends the rage and passes the turn. Focus lands on the
 * first, which loses nothing.
 */
@Component({
  selector: 'app-rage-question',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule],
  template: `
    @if (subject(); as c) {
      <div #box class="ask mr-notice mr-notice--warning" role="alertdialog" aria-labelledby="rage-q" aria-describedby="rage-d" (keydown.escape)="back.emit(c.id)">
        <mat-icon aria-hidden="true">local_fire_department</mat-icon>
        <div class="ask__body">
          <p class="ask__text" id="rage-q">
            <strong>{{ title() }}</strong>
          </p>
          <p class="ask__text" id="rage-d">
            {{ master() ? c.label + ' não atacou um hostil nem sofreu dano desde a última vez' : 'Você não atacou um hostil nem sofreu dano desde a sua última vez' }}.
            Se a fúria acabar, o turno passa.
          </p>
          <div class="ask__btns">
            <button #safe mat-stroked-button type="button" class="ask__btn" [disabled]="busy()" (click)="back.emit(c.id)">
              Voltar e atacar
            </button>
            <button mat-stroked-button type="button" class="ask__btn ask__go" [disabled]="busy()" (click)="letEnd.emit(c.id)">
              Deixar a fúria acabar
            </button>
          </div>
        </div>
      </div>
    }
  `,
  styles: `
    :host {
      display: contents;
    }

    .ask {
      align-items: flex-start;
      scroll-margin-top: 80px;
    }

    .ask__body {
      display: flex;
      flex: 1;
      flex-direction: column;
      gap: 12px;
      min-width: 0;
    }

    .ask__text {
      margin: 0;
    }

    .ask__btns {
      display: grid;
      grid-template-columns: minmax(0, 1fr);
      gap: 10px;

      @media (min-width: 768px) {
        grid-template-columns: repeat(2, 220px);
      }
    }

    .ask__btn {
      --mat-button-outlined-container-height: 48px;
      --mat-button-outlined-label-text-color: var(--mr-ink);
      --mat-button-outlined-outline-color: var(--mr-control-line);
      width: 100%;
      white-space: nowrap;

      @media (min-width: 768px) {
        --mat-button-outlined-container-height: 44px;
      }
    }
  `,
})
export class RageQuestion {
  private readonly injector = inject(Injector);

  /** The raging barbarian the turn waits for; nothing is drawn without one. */
  readonly subject = input<Combatant | null>(null);
  /** The master reads the question about someone else's rage. */
  readonly master = input(false);
  readonly busy = input(false);

  /** "Voltar e atacar": the turn stays. */
  readonly back = output<string>();
  /** "Deixar a fúria acabar": the rage ends and the turn passes. */
  readonly letEnd = output<string>();

  private readonly safe = viewChild('safe', { read: ElementRef<HTMLButtonElement> });
  private readonly box = viewChild('box', { read: ElementRef<HTMLElement> });

  protected readonly title = computed(() => {
    const c = this.subject();
    return this.master() && c ? `A fúria de ${c.label} vai acabar?` : 'A sua fúria vai acabar?';
  });

  constructor() {
    // The question opens with the focus on the safe button, and the screen scrolls to it.
    let asked = '';
    effect(() => {
      const id = this.subject()?.id ?? '';
      if (id && id !== asked) {
        afterNextRender(
          () => {
            this.box()?.nativeElement.scrollIntoView?.({ block: 'start' });
            this.safe()?.nativeElement.focus({ preventScroll: true });
          },
          { injector: this.injector },
        );
      }
      asked = id;
    });
  }
}
