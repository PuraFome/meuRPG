import { ChangeDetectionStrategy, Component, ElementRef, inject, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { KIND_LABEL, type KindChoice, type KindKey } from '../../core/images/imagegen-copy';

/**
 * The three ways to ask for a picture, as one bordered strip of radios (E10-07 1 to 3): "Arte da cena", "Vista isométrica" and "O mapa com
 * textura" ("Cena", "Isométrica" and "Textura" on a phone). The one that cannot be made from where the dialog opened is dashed and quiet, with
 * `aria-disabled` (it stays reachable, so a screen reader hears it), and the reason is written under the strip by the form: a state is never
 * colour alone. Tab reaches the chosen one; the arrows move to the next one that can be chosen.
 */
@Component({
  selector: 'app-kind-strip',
  imports: [MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="strip" role="radiogroup" aria-label="Como a imagem é feita" [attr.aria-describedby]="describedBy()" (keydown)="onKey($event)">
      @for (c of choices(); track c.key) {
        <button
          type="button"
          role="radio"
          class="seg"
          [class.seg--on]="c.key === value()"
          [class.seg--off]="!c.available"
          [attr.aria-checked]="c.key === value()"
          [attr.aria-disabled]="c.available ? null : 'true'"
          [tabindex]="c.key === value() ? 0 : -1"
          (click)="pick(c)"
        >
          @if (c.key === value()) {
            <mat-icon aria-hidden="true">check</mat-icon>
          }
          <span class="seg__long">{{ label(c.key).long }}</span>
          <span class="seg__short" aria-hidden="true">{{ label(c.key).short }}</span>
        </button>
      }
    </div>
  `,
  styleUrl: './kind-strip.scss',
})
export class KindStrip {
  readonly choices = input.required<readonly KindChoice[]>();
  readonly value = input.required<KindKey>();
  readonly describedBy = input<string | null>(null);
  readonly valueChange = output<KindKey>();

  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  protected label(key: KindKey) {
    return KIND_LABEL[key];
  }

  protected pick(c: KindChoice): void {
    if (c.available && c.key !== this.value()) {
      this.valueChange.emit(c.key);
    }
  }

  protected onKey(event: KeyboardEvent): void {
    const step = event.key === 'ArrowRight' || event.key === 'ArrowDown' ? 1 : event.key === 'ArrowLeft' || event.key === 'ArrowUp' ? -1 : 0;
    if (step === 0) {
      return;
    }
    event.preventDefault();
    const all = this.choices();
    const at = Math.max(0, all.findIndex((c) => c.key === this.value()));
    for (let n = 1; n <= all.length; n++) {
      const i = (at + step * n + all.length * n) % all.length;
      const next = all[i];
      if (next?.available) {
        this.valueChange.emit(next.key);
        setTimeout(() => this.host.nativeElement.querySelectorAll<HTMLElement>('[role="radio"]')[i]?.focus());
        return;
      }
    }
  }
}
