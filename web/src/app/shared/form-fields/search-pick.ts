import { ChangeDetectionStrategy, Component, computed, input, output, signal } from '@angular/core';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';

import { plain } from '../../core/content/content-kinds';
import { FieldNote } from './field-note';

let nextId = 0;

/** A value the search offers; `group` is the id of one of the control's groups (empty without groups). */
export interface SearchOption {
  readonly value: string;
  readonly label: string;
  /** Where it comes from ("talento do SRD"), shown after the name; often empty. */
  readonly hint: string;
  readonly group: string;
}

export interface SearchGroup {
  readonly id: string;
  readonly title: string;
  /** The line of a group with no match: "Nenhum traço com esse nome." */
  readonly empty: string;
}

/** A chosen value, as a chip that can be taken off; `tag` is a short mark on it ("Desligada"), often empty. */
export interface SearchChip {
  readonly value: string;
  readonly label: string;
  readonly hint: string;
  readonly tag: string;
}

export interface SearchSection {
  /** Null for a list with no groups. */
  readonly group: SearchGroup | null;
  readonly options: readonly SearchOption[];
}

/** The options whose name has the search (case and accents aside), under their groups; a group with none is kept, to say so. */
export function searchSections(
  options: readonly SearchOption[],
  groups: readonly SearchGroup[],
  query: string,
): SearchSection[] {
  const q = plain(query);
  const found = options.filter((o) => q === '' || plain(o.label).includes(q));
  if (groups.length === 0) {
    return [{ group: null, options: found }];
  }
  return groups.map((group) => ({ group, options: found.filter((o) => o.group === group.id) }));
}

/**
 * A search over a long closed list: type to filter, pick from the dropdown (under its groups), and the picks stand as chips
 * that can be taken off. The parent owns the values: `added` says which option was picked and `removed` which chip was taken
 * off, so a single-value field replaces its chip on `added`. The input carries the path as `data-field`, where a refusal lands.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-search-pick',
  imports: [FieldNote, MatFormFieldModule, MatIconModule, MatInputModule],
  template: `
    <mat-form-field appearance="outline" subscriptSizing="dynamic" [class.invalid]="issues().length > 0">
      <mat-label>{{ label() }}</mat-label>
      <input
        matInput
        role="combobox"
        autocomplete="off"
        aria-autocomplete="list"
        [placeholder]="placeholder()"
        [attr.aria-expanded]="open()"
        [attr.aria-controls]="listId"
        [attr.aria-activedescendant]="activeId()"
        [attr.aria-invalid]="issues().length > 0 ? 'true' : null"
        [attr.aria-describedby]="describedBy()"
        [attr.data-field]="path() || null"
        [value]="query()"
        (input)="typed($event)"
        (focus)="open.set(true)"
        (blur)="close()"
        (keydown)="key($event)"
      />
      <mat-icon matIconSuffix aria-hidden="true">search</mat-icon>
    </mat-form-field>
    @if (open()) {
      <div class="panel" role="listbox" [id]="listId" [attr.aria-label]="label()" (mousedown)="$event.preventDefault()">
        @for (s of sections(); track s.group?.id ?? '') {
          @if (s.group; as g) {
            <p class="panel__group" role="presentation">{{ g.title }}</p>
          }
          @for (o of s.options; track o.value) {
            <button type="button" class="panel__option" role="option" tabindex="-1" [id]="optionId(o)" [attr.aria-selected]="isActive(o)" [class.panel__option--active]="isActive(o)" (click)="pick(o)">
              {{ o.label }}
              @if (o.hint) {
                <span class="panel__hint">· {{ o.hint }}</span>
              }
            </button>
          } @empty {
            <p class="panel__empty" role="presentation">{{ s.group?.empty ?? emptyText() }}</p>
          }
        }
      </div>
    }
    @if (chips().length > 0) {
      <ul class="chips" [attr.aria-label]="label() + ' escolhidas'">
        @for (c of chips(); track c.value) {
          <li class="chip">
            {{ c.label }}
            @if (c.hint) {
              <span class="chip__hint">· {{ c.hint }}</span>
            }
            @if (c.tag) {
              <span class="mr-tag"><mat-icon aria-hidden="true">visibility_off</mat-icon>{{ c.tag }}</span>
            }
            <button type="button" class="chip__x" [attr.aria-label]="'Tirar ' + c.label" (click)="removed.emit(c.value)">
              <mat-icon aria-hidden="true">close</mat-icon>
            </button>
          </li>
        }
      </ul>
    }
    <app-field-note [id]="noteId" [issues]="issues()" [hint]="hint()" />
  `,
  styles: `
    :host {
      display: block;
      position: relative;
      min-width: 0;
    }

    mat-form-field {
      width: 100%;
    }

    .invalid {
      --mat-form-field-outlined-outline-color: var(--mr-danger-ink);
      --mat-form-field-outlined-hover-outline-color: var(--mr-danger-ink);
      --mat-form-field-outlined-focus-outline-color: var(--mr-danger-ink);
      --mat-form-field-outlined-label-text-color: var(--mr-danger-ink);
      --mat-form-field-outlined-focus-label-text-color: var(--mr-danger-ink);
    }

    .panel {
      position: absolute;
      z-index: 10;
      left: 0;
      right: 0;
      max-height: 320px;
      overflow-y: auto;
      box-sizing: border-box;
      padding: var(--mr-space-2) 0;
      border: 1px solid var(--mr-line);
      border-radius: var(--mr-radius-md);
      background: var(--mr-surface);
    }

    .panel__group {
      margin: 0;
      padding: var(--mr-space-2) var(--mr-space-4) 2px;
      font-size: var(--mr-font-14);
      font-weight: 600;
      color: var(--mr-ink-muted);
    }

    .panel__empty {
      margin: 0;
      padding: var(--mr-space-2) var(--mr-space-4);
      font-size: var(--mr-font-15);
      color: var(--mr-ink-muted);
    }

    .panel__option {
      display: block;
      width: 100%;
      min-height: 44px;
      box-sizing: border-box;
      padding: 0 var(--mr-space-4);
      border: 0;
      background: transparent;
      color: var(--mr-ink);
      font: inherit;
      font-size: var(--mr-font-16);
      text-align: left;
      cursor: pointer;

      &:hover,
      &--active {
        background: var(--mr-accent-soft);
      }
    }

    .panel__hint,
    .chip__hint {
      font-size: var(--mr-font-14);
      color: var(--mr-ink-muted);
    }

    .chips {
      display: flex;
      flex-wrap: wrap;
      gap: var(--mr-space-2);
      margin: var(--mr-space-2) 0 0;
      padding: 0;
      list-style: none;
    }

    .chip {
      display: inline-flex;
      align-items: center;
      gap: var(--mr-space-2);
      min-height: 46px;
      box-sizing: border-box;
      padding: 0 0 0 14px;
      border: 1px solid var(--mr-control-line);
      border-radius: var(--mr-radius-pill);
      background: var(--mr-surface);
      color: var(--mr-ink);
      font-size: var(--mr-font-16);
    }

    // The 44 px target of design.md, flush with the chip's edge.
    .chip__x {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 44px;
      height: 44px;
      padding: 0;
      border: 0;
      border-radius: 50%;
      background: transparent;
      color: var(--mr-ink-muted);
      cursor: pointer;

      &:hover {
        color: var(--mr-accent-text);
      }

      .mat-icon {
        width: 18px;
        height: 18px;
        font-size: var(--mr-font-18);
      }
    }
  `,
})
export class SearchPick {
  readonly label = input.required<string>();
  readonly placeholder = input('');
  readonly options = input.required<readonly SearchOption[]>();
  readonly groups = input<readonly SearchGroup[]>([]);
  readonly chips = input<readonly SearchChip[]>([]);
  readonly path = input('');
  readonly issues = input<readonly string[]>([]);
  readonly hint = input('');
  /** The line of a list with no groups and no match. */
  readonly emptyText = input('Nada com esse nome.');
  readonly added = output<string>();
  readonly removed = output<string>();

  protected readonly listId = `sp-l${nextId}`;
  protected readonly noteId = `sp-n${nextId++}`;
  protected readonly query = signal('');
  protected readonly open = signal(false);
  private readonly active = signal(-1);

  protected readonly sections = computed(() =>
    searchSections(this.options(), this.groups(), this.query()),
  );
  private readonly flat = computed(() => this.sections().flatMap((s) => s.options));
  protected readonly activeId = computed(() => {
    const o = this.open() ? this.flat()[this.active()] : undefined;
    return o ? this.optionId(o) : null;
  });

  protected describedBy(): string | null {
    return this.issues().length > 0 || this.hint() !== '' ? this.noteId : null;
  }

  protected optionId(o: SearchOption): string {
    return `${this.listId}-${this.flat().indexOf(o)}`;
  }

  protected isActive(o: SearchOption): boolean {
    return this.flat()[this.active()] === o;
  }

  protected typed(event: Event): void {
    this.query.set((event.target as HTMLInputElement).value);
    this.active.set(-1);
    this.open.set(true);
  }

  protected close(): void {
    this.open.set(false);
    this.active.set(-1);
  }

  protected pick(o: SearchOption): void {
    this.added.emit(o.value);
    this.query.set('');
    this.close();
  }

  protected key(event: KeyboardEvent): void {
    const n = this.flat().length;
    switch (event.key) {
      case 'ArrowDown':
      case 'ArrowUp': {
        event.preventDefault();
        this.open.set(true);
        const step = event.key === 'ArrowDown' ? 1 : -1;
        this.active.update((i) => (n === 0 ? -1 : (i + step + n) % n));
        break;
      }
      case 'Enter': {
        const o = this.open() ? this.flat()[this.active()] : undefined;
        if (o) {
          event.preventDefault();
          this.pick(o);
        }
        break;
      }
      case 'Escape':
        if (this.open()) {
          event.preventDefault();
          this.close();
        }
        break;
    }
  }
}
