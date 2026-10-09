import { ChangeDetectionStrategy, Component, effect, input, model, signal } from '@angular/core';

import { type ExtraDieField, dieFieldHint } from '../../../../core/effects/effects';

let nextId = 0;

/**
 * The d4 an effect adds to a roll when the player rolls physical dice (RN-18, RN-22: Bênção adds 1d4, Perdição takes
 * one): one field for each die, "Resultado do d4 (Bênção)", with what it is for under it and its range. The typed
 * faces go out in `faces`, in the order of the fields, `null` for an empty or out-of-range one; the page asks for
 * them only when it is about to send a roll typed from a physical die. With the app's dice nothing is drawn.
 */
@Component({
  selector: 'app-extra-dice',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (fields().length > 0) {
      <fieldset class="dice">
        <legend class="mr-visually-hidden">Dados que os efeitos somam a esta jogada</legend>
        @for (f of fields(); track $index; let i = $index) {
          <div class="die">
            <label class="die__label" [for]="id + '-' + i">{{ f.label }}</label>
            <input
              class="die__input"
              type="text"
              inputmode="numeric"
              autocomplete="off"
              [id]="id + '-' + i"
              [value]="texts()[i]"
              [attr.aria-invalid]="bad(i) ? 'true' : null"
              [attr.aria-describedby]="id + '-h-' + i"
              (input)="type(i, $event)"
            />
            <p class="die__hint" [class.die__hint--bad]="bad(i)" [id]="id + '-h-' + i">
              {{ hint(f) }} De 1 a {{ f.faces }}.
            </p>
          </div>
        }
      </fieldset>
    }
  `,
  styleUrl: './extra-dice.scss',
})
export class ExtraDice {
  /** The dice to ask for. */
  readonly fields = input<readonly ExtraDieField[]>([]);
  /** The faces typed, aligned with the fields (`null` where a field is empty or out of range). */
  readonly faces = model<readonly (number | null)[]>([]);

  protected readonly id = `extra-dice-${nextId++}`;
  protected readonly texts = signal<readonly string[]>([]);
  protected readonly hint = dieFieldHint;

  constructor() {
    // A different set of dice starts with empty fields.
    effect(() => {
      const n = this.fields().length;
      this.texts.set(Array.from({ length: n }, () => ''));
      this.faces.set(Array.from({ length: n }, () => null));
    });
  }

  protected bad(i: number): boolean {
    const text = (this.texts()[i] ?? '').trim();
    return text !== '' && this.faces()[i] === null;
  }

  protected type(i: number, event: Event): void {
    const text = (event.target as HTMLInputElement).value;
    this.texts.update((all) => all.map((t, at) => (at === i ? text : t)));
    const f = this.fields()[i];
    const n = /^\d{1,2}$/.test(text.trim()) ? Number(text.trim()) : 0;
    const face = f && n >= 1 && n <= f.faces ? n : null;
    this.faces.update((all) => all.map((v, at) => (at === i ? face : v)));
  }
}
