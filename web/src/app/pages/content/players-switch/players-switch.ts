import { Component, inject, input, output, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { TableEntry } from '../../../../gen/meurpg/rules/v1/table_content_pb';
import { TableContentClient, contentErrorText } from '../../../core/content/content-client';
import { KIND_NOUNS } from '../../../core/content/content-kinds';
import { SwitchField } from '../../../shared/form-fields/switch-field';

/**
 * "Disponível para os jogadores" (MR-025, RN-23; E10-01 states 4, 6 and 7): the entry's own switch, the same one as in "Opções
 * para os jogadores". It sits under the preview of every editor of the table's content (the spell, race, sub-race and background
 * editors; the class and subclass editors of 10.12 put `<app-players-switch>` in the same place) and saves at once, apart from
 * the form: a draft being typed is never touched, and the entry's revision does not change (it is the campaign's switch, not
 * the entry's body). Off, nobody picks it in a new sheet; the sheets that already use it keep it, and the line says how many.
 */
@Component({
  selector: 'app-players-switch',
  imports: [MatIconModule, SwitchField],
  template: `
    <section class="mr-panel panel" aria-labelledby="players-t">
      <h2 class="mr-panel__title" id="players-t">Para os jogadores</h2>
      <app-switch-field label="Disponível para os jogadores" [checked]="!entry().off" (toggled)="toggle($event)" />
      <p class="note">{{ note() }}</p>
      @if (error()) {
        <p class="error" role="alert"><mat-icon aria-hidden="true">error</mat-icon>{{ error() }}</p>
      }
      <p class="mr-visually-hidden" role="status" aria-live="polite">{{ saved() }}</p>
    </section>
  `,
  styles: `
    :host {
      display: block;
    }

    .panel {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-2);
      margin-top: var(--mr-space-4);

      .mr-panel__title {
        margin: 0;
      }
    }

    .note {
      margin: 0;
      font-size: 14px;
      line-height: 19px;
      color: var(--mr-ink-muted);
    }

    .error {
      display: flex;
      align-items: flex-start;
      gap: 6px;
      margin: 0;
      font-size: 15px;
      color: var(--mr-danger-ink);

      .mat-icon {
        flex: none;
        width: 20px;
        height: 20px;
        font-size: 20px;
      }
    }
  `,
})
export class PlayersSwitch {
  private readonly client = inject(TableContentClient);

  readonly campaignId = input.required<string>();
  readonly entry = input.required<TableEntry>();
  /** The entry with its new switch, after the server said yes. */
  readonly switched = output<TableEntry>();

  protected readonly busy = signal(false);
  protected readonly error = signal('');
  protected readonly saved = signal('');

  protected note(): string {
    const e = this.entry();
    const n = e.charactersUsing;
    const using = n === 0 ? '' : n === 1 ? ' A ficha que a usa continua funcionando.' : ` As ${n} fichas que a usam continuam funcionando.`;
    return e.off
      ? `Desligado, ninguém a escolhe numa ficha nova e os jogadores não a leem.${using || ' Quem já a usa continua com ela.'}`
      : 'Ligado, os jogadores a leem por inteiro e podem escolhê-la.';
  }

  protected async toggle(on: boolean): Promise<void> {
    const e = this.entry();
    if (this.busy() || e.off === !on) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      await this.client.setSwitches(this.campaignId(), [{ key: e.key, off: !on }]);
      const noun = KIND_NOUNS[e.kind];
      this.saved.set(`${noun?.article ?? 'A'} ${noun?.noun ?? 'entrada'} ${e.namePt} ${on ? 'está disponível' : 'não está mais disponível'} para os jogadores.`);
      this.switched.emit({ ...e, off: !on } as TableEntry);
    } catch (err) {
      this.error.set(`${contentErrorText(err, 'salvar')} O interruptor continua como estava.`);
    } finally {
      this.busy.set(false);
    }
  }
}
