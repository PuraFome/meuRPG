import { ChangeDetectionStrategy, Component, ElementRef, Injector, afterNextRender, inject, input, output, signal, viewChild } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { focusWithRing } from '../../../core/creatures/focus-ring';

let nextId = 0;

/**
 * The foot of the new point panels (E9-02): "Salvar ponto", the screen's one filled button, and "Apagar ponto"
 * beside it, two buttons of the same size. Deleting asks in place ("Apagar Fosso escondido? Não dá para
 * desfazer."), with the focus on "Cancelar". A point that cannot be deleted (a treasure that was found or turned
 * into XP) shows the dashed "can't act" button and the reason in a line. Presentational: the page runs the calls.
 */
@Component({
  selector: 'app-point-foot',
  imports: [MatButtonModule, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (confirming()) {
      <div class="pf__confirm" role="group" [attr.aria-labelledby]="id">
        <p [id]="id"><strong>Apagar {{ name() }}?</strong> Não dá para desfazer.</p>
        <div class="pf__pair">
          <button matButton="outlined" type="button" class="pf__danger" (click)="remove.emit()">Apagar ponto</button>
          <button #cancelButton matButton="outlined" type="button" (click)="cancelAsk()">Cancelar</button>
        </div>
      </div>
    } @else {
      <div class="pf__pair">
        <button matButton="filled" type="button" [class.mr-button--off]="saving() || !dirty()" [disabled]="saving() || !dirty()" disabledInteractive (click)="!saving() && dirty() && save.emit()">Salvar ponto</button>
        <button
          #askButton
          matButton="outlined"
          type="button"
          class="pf__danger"
          [class.mr-button--off]="!!blocked()"
          [disabled]="!!blocked()"
          disabledInteractive
          [attr.aria-describedby]="blocked() ? id + '-why' : null"
          (click)="!blocked() && ask()"
        >
          <mat-icon aria-hidden="true">delete</mat-icon>Apagar ponto
        </button>
      </div>
      @if (blocked(); as why) {
        <p class="pf__why" [id]="id + '-why'">{{ why }}</p>
      }
    }
  `,
  styles: `
    :host {
      display: block;
      padding-top: var(--mr-space-3);
      border-top: 1px solid var(--mr-rule);
    }

    .pf__pair {
      display: flex;
      flex-wrap: wrap;
      gap: var(--mr-space-3);
    }

    .pf__pair button {
      flex: 1 1 8rem;
      min-height: 44px;
      font-weight: 700;

      @media (max-width: 767.98px) {
        min-height: 48px;
      }
    }

    .pf__danger:not(.mr-button--off) {
      color: var(--mr-danger-ink);
      --mat-button-outlined-label-text-color: var(--mr-danger-ink);
    }

    .pf__confirm p,
    .pf__why {
      margin: 0 0 var(--mr-space-2);
      font-size: 14px;
      line-height: 19px;
    }

    .pf__confirm p {
      font-size: 16px;
      line-height: 22px;
    }

    .pf__why {
      margin: var(--mr-space-2) 0 0;
      color: var(--mr-ink-muted);
    }
  `,
})
export class PointFoot {
  private readonly injector = inject(Injector);
  protected readonly id = `pf-${nextId++}`;

  readonly name = input.required<string>();
  readonly saving = input(false);
  readonly dirty = input(false);
  /** Why the point cannot be deleted, or empty. */
  readonly blocked = input('');

  readonly save = output<void>();
  readonly remove = output<void>();

  protected readonly confirming = signal(false);
  private readonly cancelButton = viewChild('cancelButton', { read: ElementRef<HTMLButtonElement> });
  private readonly askButton = viewChild('askButton', { read: ElementRef<HTMLButtonElement> });

  protected ask(): void {
    this.confirming.set(true);
    afterNextRender(() => focusWithRing(this.cancelButton()?.nativeElement), { injector: this.injector });
  }

  protected cancelAsk(): void {
    this.confirming.set(false);
    afterNextRender(() => focusWithRing(this.askButton()?.nativeElement), { injector: this.injector });
  }
}
