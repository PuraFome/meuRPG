import { ChangeDetectionStrategy, Component, computed, input, model } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

/** One character of a list to pick from. */
export interface PickRow {
  readonly id: string;
  readonly name: string;
  /** What is said under the name: the class, the player, where the character stands. */
  readonly sub?: string;
  /** Already has it: checked, disabled, with `sub` as the reason. */
  readonly locked?: boolean;
}

/**
 * A list of characters with a checkbox each (E9-08 "Revelar para…" and "Disparar…", E9-09 "Quem
 * encontrou"): a 56 px bordered row, the checked one with a thicker accent border and a soft fill (never
 * colour alone: the box is checked), and "Marcar todos" as a text action. Nobody starts checked unless
 * the screen says so. The one who has it already shows checked and disabled with the reason.
 * Presentational: the screen owns what the pick means.
 */
@Component({
  selector: 'app-person-pick',
  imports: [MatButtonModule, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="pp__legend">
      <h3 class="pp__h" [id]="headingId()">{{ heading() }}</h3>
      @if (open().length > 1 && allToggle()) {
        <button matButton type="button" class="pp__all" (click)="toggleAll()">
          {{ allPicked() ? 'Desmarcar todos' : 'Marcar todos' }}
        </button>
      }
    </div>
    @if (rows().length === 0) {
      <p class="pp__none">{{ empty() }}</p>
    } @else {
      <ul class="pp__list" [attr.aria-labelledby]="headingId()">
        @for (r of rows(); track r.id) {
          <li>
            <label class="pp__row" [class.pp__row--on]="!r.locked && picked().has(r.id)" [class.pp__row--has]="r.locked">
              <input
                type="checkbox"
                class="mr-visually-hidden"
                [checked]="r.locked || picked().has(r.id)"
                [disabled]="r.locked"
                (change)="toggle(r.id)"
              />
              <span class="pp__box" [class.pp__box--has]="r.locked" aria-hidden="true"><mat-icon>check</mat-icon></span>
              <span class="pp__who">
                <b class="pp__name">{{ r.name }}</b>
                @if (r.sub) {
                  <span class="pp__sub">{{ r.sub }}</span>
                }
              </span>
            </label>
          </li>
        }
      </ul>
    }
  `,
  styleUrl: './person-pick.scss',
})
export class PersonPick {
  readonly rows = input.required<readonly PickRow[]>();
  readonly picked = model<ReadonlySet<string>>(new Set());
  readonly heading = input.required<string>();
  readonly headingId = input('pp-h');
  readonly empty = input('Nenhum personagem de jogador vivo nesta campanha.');
  readonly allToggle = input(true);

  protected readonly open = computed(() => this.rows().filter((r) => !r.locked));
  protected readonly allPicked = computed(
    () => this.open().length > 0 && this.open().every((r) => this.picked().has(r.id)),
  );

  protected toggle(id: string): void {
    this.picked.update((set) => {
      const next = new Set(set);
      if (!next.delete(id)) {
        next.add(id);
      }
      return next;
    });
  }

  protected toggleAll(): void {
    this.picked.set(this.allPicked() ? new Set() : new Set(this.open().map((r) => r.id)));
  }
}
