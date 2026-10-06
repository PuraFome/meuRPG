import { Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { EntrySaver } from '../../../core/content/entry-saver';

/**
 * The summary at the top of an editor (a refusal: how many fields need an adjustment, and what has no field; a stale
 * entry with "Recarregar"; a failure that is not a refusal), as an alert the screen reader reads once.
 */
@Component({
  selector: 'app-editor-alerts',
  imports: [MatButtonModule, MatIconModule],
  template: `
    @if (saver().summary(); as summary) {
      <div class="mr-notice mr-notice--danger" role="alert">
        <mat-icon aria-hidden="true">warning</mat-icon>
        <div>
          <p>
            <strong>{{ lead(summary) }}</strong> {{ rest(summary) }}
          </p>
          @if (saver().placement().top.length > 0) {
            <ul class="top">
              @for (t of saver().placement().top; track t.text) {
                <li>{{ t.text }}</li>
              }
            </ul>
          }
        </div>
      </div>
    }
    @if (saver().stale()) {
      <div class="mr-notice mr-notice--warning" role="alert">
        <mat-icon aria-hidden="true">sync_problem</mat-icon>
        <div>
          <p><strong>Esta entrada mudou enquanto você editava.</strong> Recarregue para ver a versão nova; o que você digitou aqui será perdido.</p>
          <button matButton="outlined" type="button" class="reload" (click)="reload.emit()">Recarregar</button>
        </div>
      </div>
    }
    @if (saver().error(); as error) {
      <div class="mr-notice mr-notice--danger" role="alert">
        <mat-icon aria-hidden="true">error</mat-icon>
        <p>{{ error }}</p>
      </div>
    }
  `,
  styles: `
    :host {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-3);
    }

    .top {
      margin: 6px 0 0;
      padding-left: 18px;
    }

    .reload {
      margin-top: var(--mr-space-2);
      min-height: 44px;
    }
  `,
})
export class EditorAlerts {
  readonly saver = input.required<EntrySaver>();
  readonly reload = output<void>();

  /** "Não foi possível salvar a magia." is the bold lead; the rest is the plain line. */
  protected lead(summary: string): string {
    const i = summary.indexOf('. ');
    return i < 0 ? summary : summary.slice(0, i + 1);
  }

  protected rest(summary: string): string {
    const i = summary.indexOf('. ');
    return i < 0 ? '' : summary.slice(i + 2);
  }
}

/** The foot of an editor: the one filled button, "Cancelar" and the line that says changes are live. */
@Component({
  selector: 'app-editor-bar',
  imports: [MatButtonModule, MatIconModule],
  host: { '[class.bar--sticky]': 'sticky()' },
  template: `
    <div class="bar">
      @if (blocked()) {
        <button matButton="filled" type="button" class="mr-button--off" disabled disabledInteractive [attr.aria-describedby]="'bar-reason'">
          <mat-icon aria-hidden="true">block</mat-icon>{{ saveLabel() }}
        </button>
      } @else {
        <button matButton="filled" type="button" [attr.aria-disabled]="saving()" (click)="!saving() && save.emit()">
          <mat-icon aria-hidden="true">save</mat-icon>{{ saving() ? 'Salvando...' : saveLabel() }}
        </button>
      }
      <button matButton="outlined" type="button" (click)="cancel.emit()">Cancelar</button>
      <p class="bar__note" id="bar-reason">{{ blocked() || 'Mudar vale na hora, também nas fichas travadas.' }}</p>
    </div>
  `,
  styles: `
    :host {
      display: block;
    }

    // A long editor (the class's) keeps the save bar at the foot while the page scrolls.
    :host(.bar--sticky) {
      position: sticky;
      bottom: 0;
      z-index: 5;
      margin: 0 calc(var(--mr-gutter) * -1);
      padding: var(--mr-space-3) var(--mr-gutter);
      border-top: 1px solid var(--mr-line);
      background: var(--mr-surface);
    }

    .bar {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: var(--mr-space-3);
    }

    .bar__note {
      margin: 0;
      font-size: 15px;
      color: var(--mr-ink-muted);
    }

    button {
      min-height: 44px;
    }
  `,
})
export class EditorBar {
  readonly saveLabel = input.required<string>();
  /** Stuck to the foot of the screen while the page scrolls. */
  readonly sticky = input(false);
  readonly saving = input(false);
  /** Why saving is off right now, or "". */
  readonly blocked = input('');
  readonly save = output<void>();
  readonly cancel = output<void>();
}
