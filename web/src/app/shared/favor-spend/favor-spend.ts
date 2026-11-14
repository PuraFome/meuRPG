import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { FavorSpend } from '../../core/play/favor-spend';

let nextId = 0;

/**
 * "Inspiração do mestre": the box that spends the master's Inspiração on one roll (RN-43, boards 581-03 and 581-07), above "Como
 * rolar o d20" or the roll buttons. The sheet owns a `FavorSpend` and gives it here; this only draws it. Ticked, it is a card with
 * a border and a check (never colour alone). Off, it is dashed and still reachable: the click does nothing, `aria-disabled` says
 * so and the reason is linked with `aria-describedby`. Nothing is drawn for a character that has none, for an NPC and while the
 * read has not come.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-favor-spend',
  imports: [MatIconModule],
  templateUrl: './favor-spend.html',
  styleUrl: './favor-spend.scss',
})
export class FavorSpendBox {
  readonly control = input.required<FavorSpend>();
  /** A request is in the air: the box is as it was sent. */
  readonly busy = input(false);

  protected readonly id = `favor-${nextId++}`;
  protected readonly view = computed(() => this.control().view());
  protected readonly off = computed(() => this.view()?.kind === 'off' || this.busy());
  protected readonly on = computed(() => this.control().spend());
  /** What a screen reader hears after the label: the hint (which says "O mestre agindo por X" when off) and, when off, the reason. */
  protected readonly describedBy = computed(() =>
    this.view()?.kind === 'off' ? `${this.id}-hint ${this.id}-why` : `${this.id}-hint`,
  );
  protected readonly sub = computed(() => {
    const v = this.view();
    return v?.kind === 'off' && v.sub !== ''
      ? v.sub
      : 'Vantagem nesta jogada · ela sai da sua ficha';
  });
  protected readonly reason = computed(() => {
    const v = this.view();
    return v?.kind === 'off' ? v.reason : '';
  });

  protected toggle(event: Event): void {
    if (this.off()) {
      event.preventDefault();
      return;
    }
    this.control().set((event.target as HTMLInputElement).checked);
  }
}
