import { Component, computed, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { type GridColumns, type RowDraft, circleLabel } from '../../core/content/class-draft';

/** A feature chip in a row of the grid: a name the master opens, or one the app puts there itself (locked). */
export interface GridChip {
  readonly id: string;
  readonly label: string;
  readonly locked: boolean;
}

/** One row the grid draws: the class level, the path its inputs carry, the numbers, the chips of its features. */
export interface GridRowVm {
  readonly level: number;
  /** "table_class.levels[4]": the inputs are `${base}.prof_bonus`, `${base}.cantrips_known`, `${base}.spells_known`, `${base}.slots[2]`. */
  readonly base: string;
  readonly row: RowDraft;
  readonly chips: readonly GridChip[];
}

export type GridField = 'profBonus' | 'cantrips' | 'spells' | 'slot';

export interface GridEdit {
  readonly level: number;
  readonly field: GridField;
  /** For `slot`: the circle, 0 for the 1st. */
  readonly slot: number;
  readonly value: number;
}

let nextGridId = 0;

/** What a cell says when it is read aloud: "Nível 5, espaços de magia de 2º nível". */
function fieldLabel(field: GridField, slot: number): string {
  switch (field) {
    case 'profBonus':
      return 'bônus de proficiência';
    case 'cantrips':
      return 'truques';
    case 'spells':
      return 'magias conhecidas';
    default:
      return `espaços de magia de ${circleLabel(slot)} nível`;
  }
}

/** A refusal at a cell or a row, with where it is in words. */
export interface GridProblem {
  readonly where: string;
  readonly text: string;
}

/**
 * The 20-level table (MR-025, E10-02 state 2): one row per level, the numbers an input each, and the grid scrolls inside itself
 * (a fixed height, the column titles stuck on top, the level stuck on the left when it scrolls sideways). It draws and edits
 * what it is given and works nothing out: the numbers come from the server's defaults and go back as the master typed them. Tab
 * goes cell by cell; the arrow keys change the row, Page Down skips 5 levels. Every input carries its path as `data-field`, so
 * the refusal of the server lands on the cell.
 */
@Component({
  selector: 'app-level-grid',
  imports: [MatIconModule],
  templateUrl: './level-grid.html',
  styleUrl: './level-grid.scss',
})
export class LevelGrid {
  readonly rows = input.required<readonly GridRowVm[]>();
  readonly columns = input.required<GridColumns>();
  /** The proficiency bonus column (a class has it; the third caster's table of a subclass does not). */
  readonly showBonus = input(true);
  /** The features column, with a "+" in each row. */
  readonly showFeatures = input(true);
  readonly label = input('Tabela dos 20 níveis');
  /** The path of the grid itself (a refusal about the whole table: "a tabela precisa dos 20 níveis"). */
  readonly path = input('');
  readonly issuesOf = input<(path: string) => readonly string[]>(() => []);

  readonly edited = output<GridEdit>();
  readonly chipOpened = output<string>();
  readonly featureAdded = output<number>();

  protected readonly problemsId = `grid-problems-${nextGridId++}`;
  protected readonly circles = computed(() => Array.from({ length: this.columns().circles }, (_, i) => i));

  /** Every refusal at a cell or a row, with its place in words (shown under the grid, so a cell stays small). */
  protected readonly problems = computed<GridProblem[]>(() => {
    const out: GridProblem[] = [];
    const issues = this.issuesOf();
    const cols = this.columns();
    for (const r of this.rows()) {
      const at = (path: string, what: string): void => {
        for (const text of issues(path)) out.push({ where: `Nível ${r.level}${what ? `, ${what}` : ''}`, text });
      };
      at(r.base, '');
      if (this.showBonus()) at(`${r.base}.prof_bonus`, fieldLabel('profBonus', 0));
      if (cols.cantrips) at(`${r.base}.cantrips_known`, fieldLabel('cantrips', 0));
      if (cols.spells) at(`${r.base}.spells_known`, fieldLabel('spells', 0));
      at(`${r.base}.slots`, 'espaços de magia');
      for (let i = 0; i < 9; i++) at(`${r.base}.slots[${i}]`, fieldLabel('slot', i));
    }
    return out;
  });

  protected readonly tableIssues = computed(() => (this.path() ? this.issuesOf()(this.path()) : []));

  protected cellLabel(level: number, field: GridField, slot = 0): string {
    return `Nível ${level}, ${fieldLabel(field, slot)}`;
  }

  protected bad(base: string, field: string): boolean {
    return this.issuesOf()(`${base}.${field}`).length > 0;
  }

  protected bonusText(n: number): string {
    return n > 0 ? `+${n}` : '';
  }

  protected numText(n: number): string {
    return n > 0 ? String(n) : '';
  }

  protected cellFieldPath(base: string, field: GridField, slot: number): string {
    switch (field) {
      case 'profBonus':
        return `${base}.prof_bonus`;
      case 'cantrips':
        return `${base}.cantrips_known`;
      case 'spells':
        return `${base}.spells_known`;
      default:
        return `${base}.slots[${slot}]`;
    }
  }

  protected edit(event: Event, level: number, field: GridField, slot = 0): void {
    const el = event.target as HTMLInputElement;
    const digits = el.value.replace(/\D/g, '').slice(0, 3);
    const value = digits === '' ? 0 : Number(digits);
    const shown = field === 'profBonus' ? this.bonusText(value) : this.numText(value);
    if (el.value !== shown) {
      el.value = shown;
    }
    this.edited.emit({ level, field, slot, value });
  }

  /** The arrow keys move between rows of the same column, Page Down and Page Up by 5 levels. */
  protected key(event: KeyboardEvent): void {
    const el = event.target as HTMLElement;
    if (!(el instanceof HTMLInputElement)) {
      return;
    }
    const step = event.key === 'ArrowDown' ? 1 : event.key === 'ArrowUp' ? -1 : event.key === 'PageDown' ? 5 : event.key === 'PageUp' ? -5 : 0;
    if (step === 0) {
      return;
    }
    const col = el.dataset['col'];
    const row = Number(el.dataset['row']);
    const root = el.closest('table');
    const rows = this.rows().length;
    const target = Math.max(0, Math.min(rows - 1, row + step));
    const next = root?.querySelector<HTMLInputElement>(`input[data-col="${col}"][data-row="${target}"]`);
    if (next) {
      event.preventDefault();
      next.focus();
      next.select();
    }
  }
}
