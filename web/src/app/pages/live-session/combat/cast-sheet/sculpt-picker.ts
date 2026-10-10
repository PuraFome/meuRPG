import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

let nextId = 0;

/** A creature the caster may spare with Sculpt Spells. */
export interface SculptRow {
  readonly id: string;
  readonly label: string;
  /** On the caster's side: listed first. */
  readonly ally: boolean;
}

/**
 * "Esculpir Magias" on the cast sheet (SRD 5.1, School of Evocation wizard): after the area is placed and the creatures
 * in it are listed, the caster may pick up to 1 + the spell's level of them. Those pass the saving throw by themselves
 * and take no damage when the spell would deal half on a success. Nothing is marked at first: the player chooses.
 * Real checkboxes: Space marks. At the limit, the unmarked ones are disabled and the counter says why.
 */
@Component({
  selector: 'app-sculpt-picker',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    <fieldset class="sc" [attr.aria-describedby]="id + '-d'">
      <legend class="sc__title">
        Esculpir Magias
        <span class="sc__count" aria-live="polite">· {{ chosen().length }} de {{ limit() }}</span>
      </legend>
      <p class="sc__help" [id]="id + '-d'">
        Escolha até {{ limit() }} {{ limit() === 1 ? 'criatura' : 'criaturas' }} que passam automaticamente e não sofrem
        dano.
      </p>
      @for (r of rows(); track r.id) {
        <label class="check" [class.check--off]="atLimit() && !chosen().includes(r.id)">
          <input
            type="checkbox"
            [checked]="chosen().includes(r.id)"
            [disabled]="atLimit() && !chosen().includes(r.id)"
            (change)="pick.emit(r.id)"
          />
          <span>{{ r.label }}@if (r.ally) {<span class="check__tag"> · aliado</span>}</span>
        </label>
      }
    </fieldset>
  `,
  styles: `
    :host {
      display: block;
    }

    .sc {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-1);
      margin: 0;
      padding: 0;
      border: 0;
    }

    .sc__title {
      margin: 0 0 var(--mr-space-1);
      padding: 0;
      font-size: 15px;
      line-height: 20px;
      font-weight: 700;
    }

    .sc__count {
      font-weight: 400;
      color: var(--mr-ink-muted);
    }

    .sc__help {
      margin: 0 0 var(--mr-space-1);
      font-size: 14px;
      line-height: 19px;
      color: var(--mr-ink-muted);
    }

    .check {
      display: flex;
      align-items: center;
      gap: var(--mr-space-2);
      min-height: 44px;

      &--off {
        color: var(--mr-ink-muted);
      }
    }

    .check__tag {
      color: var(--mr-ink-muted);
    }
  `,
})
export class SculptPicker {
  readonly rows = input.required<readonly SculptRow[]>();
  readonly chosen = input.required<readonly string[]>();
  /** 1 + the level the spell is cast at (a cantrip is 1). */
  readonly limit = input.required<number>();

  readonly pick = output<string>();

  protected readonly id = `sculpt-${nextId++}`;
  protected readonly atLimit = computed(() => this.chosen().length >= this.limit());
}
