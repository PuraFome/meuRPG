import { ChangeDetectionStrategy, Component, computed, signal } from '@angular/core';

import { chooserLine } from '../../../core/conversation/message-view';
import { injectSheet } from '../../../shared/sheet/sheet-host';
import { SheetFrame } from '../../../shared/sheet/sheet-frame/sheet-frame';
import type { ChooserPlayer } from './conversation-roster';

export interface AudienceData {
  readonly players: readonly ChooserPlayer[];
  /** The accounts that read now; empty is the whole table. */
  readonly selected: readonly string[];
}

/**
 * "Quem lê esta mensagem?" (MR-051): the master's chooser of the readers of a line, a bottom sheet
 * on a phone and a dialog from a tablet up (`openSheet`). It opens with the readers now chosen;
 * "Cancelar" keeps them, "Pronto" answers the accounts, in the table's order, or none for the
 * whole table. The live line says in words who reads, and the note says what the master is told
 * before sending: he always reads, and anyone not chosen never sees the line.
 */
@Component({
  selector: 'app-audience-chooser',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [SheetFrame],
  template: `
    <app-sheet-frame
      title="Quem lê esta mensagem?"
      titleId="audience-title"
      subtitle="Você sempre lê. Quem não for escolhido não vê a mensagem nem sabe que ela existe."
      [phone]="sheet.inSheet"
      (closed)="cancel()"
    >
      <fieldset class="group">
        <legend class="mr-visually-hidden">Para quem</legend>
        <label class="choice" [class.choice--on]="mode() === 'all'">
          <input type="radio" name="audience" [checked]="mode() === 'all'" (change)="mode.set('all')" />
          <span>Toda a mesa</span>
        </label>
        <label class="choice" [class.choice--on]="mode() === 'some'">
          <input type="radio" name="audience" [checked]="mode() === 'some'" (change)="mode.set('some')" />
          <span>Só algumas pessoas</span>
        </label>
      </fieldset>
      <fieldset class="group group--people">
        <legend class="mr-label">Escolha quem lê</legend>
        @for (p of players; track p.userId) {
          <label class="person">
            <input
              type="checkbox"
              [checked]="checked().has(p.userId)"
              [disabled]="mode() === 'all'"
              (change)="toggle(p.userId)"
            />
            <span class="person__name">{{ p.name }}</span>
            <span class="person__sub">{{ p.sub }}</span>
          </label>
        }
      </fieldset>
      <p class="live" role="status">{{ line() }}</p>
      <div foot class="foot">
        <button type="button" class="foot__button" (click)="cancel()">Cancelar</button>
        <button
          type="button"
          class="foot__button foot__button--primary"
          [attr.aria-disabled]="empty() ? 'true' : null"
          (click)="done()"
        >
          Pronto
        </button>
      </div>
    </app-sheet-frame>
  `,
  styleUrl: './audience-chooser.scss',
})
export class AudienceChooser {
  protected readonly sheet = injectSheet<AudienceData, readonly string[] | undefined>();
  protected readonly players = this.sheet.data.players;
  protected readonly mode = signal<'all' | 'some'>(
    this.sheet.data.selected.length > 0 ? 'some' : 'all',
  );
  protected readonly checked = signal<ReadonlySet<string>>(new Set(this.sheet.data.selected));

  /** The chosen accounts in the table's order. */
  private readonly chosen = computed(() =>
    this.players.filter((p) => this.checked().has(p.userId)),
  );
  protected readonly empty = computed(() => this.mode() === 'some' && this.chosen().length === 0);
  protected readonly line = computed(() =>
    chooserLine(
      this.chosen().map((p) => p.name),
      this.mode() === 'all',
    ),
  );

  protected toggle(userId: string): void {
    this.checked.update((set) => {
      const next = new Set(set);
      if (!next.delete(userId)) {
        next.add(userId);
      }
      return next;
    });
  }

  protected cancel(): void {
    this.sheet.close(undefined);
  }

  protected done(): void {
    if (this.empty()) {
      return;
    }
    this.sheet.close(this.mode() === 'all' ? [] : this.chosen().map((p) => p.userId));
  }
}
