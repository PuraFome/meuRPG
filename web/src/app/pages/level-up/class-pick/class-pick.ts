import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  computed,
  inject,
  input,
  output,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { article } from '../../../core/combat/combat-log';

/** A class of the sheet, as the cards show it: its name and the level it would go from and to. */
export interface ClassOption {
  readonly key: string;
  readonly name: string;
  readonly from: number;
  readonly to: number;
}

const COUNT = [
  'duas',
  'três',
  'quatro',
  'cinco',
  'seis',
  'sete',
  'oito',
  'nove',
  'dez',
  'onze',
  'doze',
];

/**
 * "Qual classe sobe de nível?": the first panel of the guided level-up when the sheet has two or more classes
 * (MR-040). One radio card per class of the sheet, each with its name and "nível 3 → 4"; the checked one is the class
 * the options below were read for. A new class is multiclassing, which stays in the sheet editor, so only the classes
 * the sheet has are offered. Tapping the other card after choices were made does not switch at once: the question
 * "Trocar de classe?" opens in place, with the safe button, "Continuar com ...", first to get the focus.
 * The page reads the options and runs the switch; this draws the cards and the question.
 */
@Component({
  selector: 'app-class-pick',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './class-pick.html',
  styleUrl: './class-pick.scss',
})
export class ClassPick {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  readonly classes = input.required<readonly ClassOption[]>();
  /** The class the level goes to now. */
  readonly selected = input.required<string>();
  /** The character's name, for "A Corvina tem duas classes". */
  readonly characterName = input.required<string>();
  /** The character's total level after the level-up. */
  readonly totalToLevel = input.required<number>();
  /** The options of the other class are being read: the cards wait. */
  readonly busy = input(false);
  /** The class the person tapped while choices were made: the question is open for it. */
  readonly asking = input<string | null>(null);
  /** The words a screen reader hears when the class changes. */
  readonly status = input('');

  readonly pick = output<string>();
  readonly confirmSwitch = output<void>();
  readonly keep = output<void>();

  protected readonly lead = computed(() => {
    const who = this.characterName();
    const the = `${article(who) === 'a' ? 'A' : 'O'} ${who}`;
    const n = this.classes().length;
    return `${the} tem ${COUNT[n - 2] ?? n} classes. O nível ${this.totalToLevel()} entra em uma delas; o resto da ficha não muda.`;
  });
  protected readonly asked = computed(() => this.classes().find((c) => c.key === this.asking()));
  protected readonly current = computed(() =>
    this.classes().find((c) => c.key === this.selected()),
  );

  protected choose(key: string): void {
    this.pick.emit(key);
    // A card the page did not take (the question opened instead) goes back to the class that is still in force.
    queueMicrotask(() => this.syncRadios());
    setTimeout(() => this.syncRadios());
  }

  /** The checked radio always follows `selected`, whatever the person tapped. */
  private syncRadios(): void {
    this.host.nativeElement
      .querySelectorAll<HTMLInputElement>('input[type=radio]')
      .forEach((i) => (i.checked = i.value === this.selected()));
  }

  /** The focus goes to the checked card: where the page opens, and where it returns after a switch. */
  focusSelected(): void {
    this.host.nativeElement.querySelector<HTMLInputElement>('input[type=radio]:checked')?.focus();
  }
}
