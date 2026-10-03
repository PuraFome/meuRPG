import {
  Component,
  ElementRef,
  computed,
  input,
  signal,
  viewChild,
} from '@angular/core';
import { toObservable, toSignal } from '@angular/core/rxjs-interop';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelect, MatSelectModule } from '@angular/material/select';
import { startWith, switchMap } from 'rxjs';

import { formatInt, formatXp, tight } from '../../../core/format/text';
import type { ChallengeRatingVm } from '../character-editor.types';

/** The height of the sticky "mais … opções" line. */
const FOOTER_HEIGHT = 32;

/**
 * "Nível de desafio (ND)" and "XP ao derrotar" of an enemy, a boss or a
 * minion (E7-11, MR-016, D1): the master picks the ND from the SRD table and
 * the XP fills in from it. Changing the ND always refills the XP, even over a
 * value the master typed; while the ND does not change, a typed value stays.
 * "Usar 50 XP" puts the table's value back (and the focus on the field, since
 * the button that had it is gone). 0 XP is a choice: "Este NPC não dá XP".
 *
 * Only the master sees this, and a player character's editor never shows it
 * (the editor decides). The two controls are the editor's own; this only
 * draws them and keeps them in step. The open list is 44px a row, with the
 * XP beside each ND and a quiet "mais 28 opções, até ND 30" under the last
 * visible row until the end is reached.
 */
@Component({
  selector: 'app-defeat-xp',
  imports: [MatButtonModule, MatFormFieldModule, MatInputModule, MatSelectModule, ReactiveFormsModule],
  templateUrl: './defeat-xp.html',
  styleUrl: './defeat-xp.scss',
})
export class DefeatXp {
  readonly rating = input.required<FormControl<string>>();
  readonly xp = input.required<FormControl<number>>();
  readonly table = input.required<readonly ChallengeRatingVm[]>();

  private readonly xpField = viewChild<ElementRef<HTMLInputElement>>('xpInput');
  private readonly select = viewChild(MatSelect);
  protected readonly remaining = signal(0);
  private panelScroll: (() => void) | null = null;

  protected readonly ratingValue = toSignal(
    toObservable(this.rating).pipe(switchMap((c) => c.valueChanges.pipe(startWith(c.value)))),
    { initialValue: '' },
  );
  private readonly xpValue = toSignal(
    toObservable(this.xp).pipe(switchMap((c) => c.valueChanges.pipe(startWith(c.value)))),
    { initialValue: 0 },
  );

  /** What the table gives the chosen ND, or `null` for no ND. */
  protected readonly tableXp = computed<number | null>(
    () => this.table().find((row) => row.rating === this.ratingValue())?.xp ?? null,
  );
  protected readonly isTable = computed(() => this.tableXp() !== null && this.xpValue() === this.tableXp());
  protected readonly hasRating = computed(() => this.ratingValue() !== '');
  protected readonly tableText = computed(() => {
    const xp = this.tableXp();
    return xp === null ? '' : formatXp(xp);
  });
  protected readonly xpHint = computed(() => {
    const table = this.tableXp();
    if (table === null) {
      return 'Escolha o ND para ver o XP da tabela.';
    }
    const base = tight(`Da tabela: ${formatInt(table)} XP.`);
    if (this.xpValue() === table) {
      return `${base} O mestre pode digitar outro valor.`;
    }
    return this.xpValue() === 0 ? `Este NPC não dá XP. ${base}` : `${base} Você digitou outro valor.`;
  });
  protected readonly moreText = computed(() => {
    const n = this.remaining();
    return n === 1 ? 'mais 1 opção, até ND 30' : `mais ${n} opções, até ND 30`;
  });

  /** The choice of an ND is the master's: the XP follows the table. */
  protected pick(): void {
    const table = this.tableXp();
    if (table !== null) {
      this.xp().setValue(table);
    }
  }

  /** "Usar 50 XP": the button is gone after it, so the focus goes to the field. */
  protected useTable(): void {
    const table = this.tableXp();
    if (table !== null) {
      this.xp().setValue(table);
      this.xp().markAsDirty();
      this.xpField()?.nativeElement.focus();
    }
  }

  protected ratingText(row: ChallengeRatingVm): string {
    return `ND ${row.rating}`;
  }

  protected xpText(row: ChallengeRatingVm): string {
    return formatXp(row.xp);
  }

  /** The list is open: count what is still below the fold, and keep counting while it scrolls. */
  protected opened(open: boolean): void {
    if (!open) {
      this.panelScroll = null;
      return;
    }
    // The panel is in the overlay a moment after the event.
    setTimeout(() => this.watchPanel(), 0);
  }

  private watchPanel(): void {
    const panel = this.select()?.panel?.nativeElement as HTMLElement | undefined;
    if (!panel) {
      return;
    }
    const count = () => {
      // What the "mais … opções" line stands for: the rows that start below the
      // visible part (the line itself covers the last 32px of it).
      const fold = panel.getBoundingClientRect().bottom - FOOTER_HEIGHT;
      const options = Array.from(panel.querySelectorAll<HTMLElement>('.nd-option'));
      this.remaining.set(options.filter((o) => o.getBoundingClientRect().top >= fold).length);
    };
    this.panelScroll = count;
    panel.addEventListener('scroll', () => this.panelScroll?.(), { passive: true });
    count();
  }
}
