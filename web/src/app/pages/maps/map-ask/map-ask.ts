import { ChangeDetectionStrategy, Component, ElementRef, Injector, afterNextRender, inject, input, output, viewChild } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';

import { focusWithRing } from '../../../core/creatures/focus-ring';

let nextId = 0;

/**
 * An in-place question of the map editor (E9-01 4: "Mudar a grade?", "Trocar a imagem?", "Esquecer o que foi
 * visto?"): it opens inside the panel of whoever asked, with the focus on its title, "Voltar" first (outlined)
 * and the one filled button under it, which says what it erases. Nothing is erased before the second click;
 * Esc closes it. Presentational: the screen puts the words and the field between the title and the buttons,
 * runs the call, and puts the focus back on the button that asked. Both buttons are as wide as the room, 44 px
 * high (48 px on a phone).
 */
@Component({
  selector: 'app-map-ask',
  imports: [MatButtonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="ask" role="group" [attr.aria-labelledby]="id" (keydown.escape)="cancel.emit()">
      <h3 #titleEl class="ask__title" [id]="id" tabindex="-1">{{ title() }}</h3>
      <ng-content />
      <div class="ask__pair">
        <button matButton="outlined" type="button" (click)="cancel.emit()">Voltar</button>
        <button
          matButton="filled"
          type="button"
          [class.mr-button--off]="!ready()"
          [disabled]="busy() || !ready()"
          disabledInteractive
          (click)="!busy() && ready() && confirm.emit()"
        >
          {{ confirmLabel() }}
        </button>
      </div>
    </section>
  `,
  styles: `
    :host {
      display: block;
    }

    .ask {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-3);
      scroll-margin-top: calc(64px + var(--mr-space-3));
    }

    .ask__title {
      margin: 0;
      font-family: var(--mr-font-display);
      font-size: 22px;
      font-weight: 700;
      line-height: 28px;
    }

    .ask__title:focus-visible,
    .ask__title[data-ring] {
      outline: 2px solid var(--mr-focus);
      outline-offset: 2px;
    }

    .ask__pair {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-2);
    }

    .ask__pair button {
      width: 100%;
      min-height: 44px;

      @media (max-width: 767.98px) {
        min-height: 48px;
      }
    }
  `,
})
export class MapAsk {
  private readonly injector = inject(Injector);
  protected readonly id = `ask-${nextId++}`;
  private readonly heading = viewChild.required<ElementRef<HTMLElement>>('titleEl');

  readonly title = input.required<string>();
  readonly confirmLabel = input.required<string>();
  /** The filled button can be pressed; false draws the dashed one (the reason is a sentence above). */
  readonly ready = input(true);
  readonly busy = input(false);
  readonly cancel = output<void>();
  readonly confirm = output<void>();

  constructor() {
    afterNextRender(
      () => {
        const el = this.heading().nativeElement;
        el.scrollIntoView?.({ block: 'nearest' });
        focusWithRing(el);
      },
      { injector: this.injector },
    );
  }
}
