import { Component, computed, input, model, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

let nextId = 0;

/**
 * One number of the adjust sheet (E5-05): the steppers around a field
 * where typing a value sets it. `steps` are the button sizes: `[5, 1]`
 * gives "−5 −1 [field] +1 +5" (the hit points row, `layout="wide"`), `[1]`
 * gives "− [field] +" on the right of the label (`layout="row"`).
 *
 * A button at a limit is disabled, and the words next to the label say why
 * ("máximo 24", "0 de 3 usados"). A typed value outside the limits is not
 * changed behind the person's back: the field says what's allowed, and the
 * sheet doesn't save until it's fixed.
 *
 * `value` is `NaN` while the field holds something that isn't a whole
 * number.
 */
@Component({
  selector: 'app-vitals-stepper',
  imports: [MatIconModule],
  templateUrl: './vitals-stepper.html',
  styleUrl: './vitals-stepper.scss',
})
export class VitalsStepper {
  readonly value = model.required<number>();
  readonly label = input.required<string>();
  /** The words under (row) or right of (wide) the label. */
  readonly hint = input('');
  readonly min = input(0);
  readonly max = input.required<number>();
  readonly steps = input<readonly number[]>([1]);
  readonly layout = input<'wide' | 'row'>('row');
  /** The field's accessible name ("Pontos de vida atuais"). */
  readonly fieldLabel = input.required<string>();
  /** A button's accessible name, from its signed step ("Tirar 5 PV"). */
  readonly stepLabel = input.required<(step: number) => string>();

  protected readonly id = `vitals-stepper-${nextId++}`;
  /** What the field shows: the person's typing, or the last stepped value. */
  protected readonly raw = signal<string | null>(null);

  protected readonly decrements = computed(() => this.steps().map((s) => -s));
  protected readonly increments = computed(() => [...this.steps()].reverse());

  readonly valid = computed(() => {
    const v = this.value();
    return Number.isInteger(v) && v >= this.min() && v <= this.max();
  });

  protected readonly display = computed(() => this.raw() ?? String(this.value()));

  protected canStep(step: number): boolean {
    const v = this.value();
    if (!Number.isInteger(v)) {
      return true;
    }
    return step < 0 ? v > this.min() : v < this.max();
  }

  protected stepBy(step: number): void {
    const v = this.value();
    const base = Number.isInteger(v) ? v : this.min();
    const next = Math.max(this.min(), Math.min(this.max(), base + step));
    this.raw.set(null);
    this.value.set(next);
  }

  protected typed(text: string): void {
    this.raw.set(text);
    this.value.set(/^\d{1,4}$/.test(text.trim()) ? Number(text.trim()) : NaN);
  }

  protected signed(step: number): string {
    return step < 0 ? `−${-step}` : `+${step}`;
  }
}
