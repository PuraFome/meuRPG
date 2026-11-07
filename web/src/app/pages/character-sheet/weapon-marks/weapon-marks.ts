import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  afterNextRender,
  computed,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';

import type { GearVm, MarksVm } from '../../../core/characters/gear';
import type { WeaponMarkVm } from '../character-sheet.types';

type MarkName = keyof MarksVm;

/** The three marks, with the words the grid uses and when each one matters (SRD 5.1: resistance and immunity to
 * "nonmagical attacks", "silvered" and "adamantine" weapons). */
const MARKS: readonly {
  readonly name: MarkName;
  readonly word: string;
  readonly against: string;
}[] = [
  {
    name: 'silvered',
    word: 'Prateada',
    against: 'Vale contra quem resiste a armas não prateadas.',
  },
  {
    name: 'adamantine',
    word: 'Adamantina',
    against: 'Vale contra quem resiste a armas não adamantinas.',
  },
  { name: 'magical', word: 'Mágica', against: 'Vale contra quem resiste a ataques não mágicos.' },
];

/**
 * "Marcar as armas": one row for each weapon of the sheet and three toggles, "Prateada", "Adamantina" and "Mágica". It
 * edits a copy; "Salvar" hands the marks that are set (a weapon with none is plain steel and has no mark) to the page,
 * which saves them over the whole sheet.
 */
@Component({
  selector: 'app-weapon-marks',
  imports: [MatButtonModule],
  templateUrl: './weapon-marks.html',
  styleUrl: './weapon-marks.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WeaponMarks {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  /** The weapons of the sheet (equipped or carried), in the server's order. */
  readonly weapons = input.required<readonly GearVm[]>();
  /** The marks the sheet has now. */
  readonly marks = input.required<readonly WeaponMarkVm[]>();
  readonly saving = input(false);
  /** Why the last save did not go through (a stale sheet, a refusal), in words. */
  readonly error = input('');

  readonly saved = output<WeaponMarkVm[]>();
  readonly cancelled = output<void>();

  protected readonly options = MARKS;
  /** The draft, by weapon: starts as the sheet's marks. */
  private readonly draft = signal<Record<string, MarksVm> | null>(null);

  constructor() {
    afterNextRender(() => this.host.nativeElement.querySelector<HTMLElement>('input')?.focus());
  }

  private readonly current = computed(() => {
    const own = this.draft();
    if (own) {
      return own;
    }
    return Object.fromEntries(this.marks().map((m) => [m.weaponKey, m]));
  });

  protected isOn(key: string, mark: MarkName): boolean {
    return this.current()[key]?.[mark] ?? false;
  }

  protected toggle(key: string, mark: MarkName, on: boolean): void {
    const base = this.current()[key] ?? { silvered: false, adamantine: false, magical: false };
    this.draft.set({ ...this.current(), [key]: { ...base, [mark]: on } });
  }

  protected save(): void {
    const current = this.current();
    const out = this.weapons()
      .map((w) => ({
        weaponKey: w.key,
        ...(current[w.key] ?? { silvered: false, adamantine: false, magical: false }),
      }))
      .filter((m) => m.silvered || m.adamantine || m.magical);
    this.saved.emit(out);
  }
}
