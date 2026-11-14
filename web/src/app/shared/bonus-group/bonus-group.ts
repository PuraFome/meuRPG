import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  effect,
  inject,
  input,
  output,
  untracked,
} from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

let nextId = 0;

/** "Nenhum", "+1", "+2", "+3": the value each word stands for. */
const OPTIONS: readonly { readonly value: number; readonly word: string }[] = [
  { value: 0, word: 'Nenhum' },
  { value: 1, word: '+1' },
  { value: 2, word: '+2' },
  { value: 3, word: '+3' },
];

/**
 * The magic bonus of a piece as a radio group, "Bônus mágico": four equal segments in one bordered row.
 * The chosen one has a check and the soft accent; `disabled` mutes the row and makes it inert while a save runs.
 * A radio that had the focus gets it back when the row is enabled again (a refused save).
 */
@Component({
  selector: 'app-bonus-group',
  imports: [MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './bonus-group.html',
  styleUrl: './bonus-group.scss',
})
export class BonusGroup {
  /** The bonus now, 0 to 3; 0 is no bonus. */
  readonly value = input(0);
  readonly disabled = input(false);
  readonly valueChange = output<number>();

  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);
  /** The radio that had the focus last, until the focus goes to something else. */
  private focused: HTMLInputElement | null = null;
  private wasDisabled = false;

  constructor() {
    effect(() => {
      const off = this.disabled();
      untracked(() => {
        if (this.wasDisabled && !off && this.focused) {
          const radio = this.focused;
          afterNextRender(
            () => {
              const active = radio.ownerDocument.activeElement;
              if (!active || active === radio.ownerDocument.body) {
                radio.focus();
              }
            },
            { injector: this.injector },
          );
        }
        this.wasDisabled = off;
      });
    });
    const el = this.host.nativeElement;
    el.addEventListener('focusin', (e) => {
      this.focused = e.target instanceof HTMLInputElement ? e.target : null;
    });
    el.addEventListener('focusout', (e) => {
      if (e.relatedTarget) {
        this.focused = null;
      }
    });
  }

  protected readonly options = OPTIONS;
  protected readonly name = `bonus-group-${nextId++}`;
}
