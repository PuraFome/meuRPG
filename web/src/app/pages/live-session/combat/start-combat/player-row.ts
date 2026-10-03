import { Component, computed, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { combatantInitial } from '../../../../core/combat/combat-view';
import { CombatantToken } from '../../../../shared/combatant-token/combatant-token';

/** A party member's row in "Iniciar combate": a checkbox, the character, and
 * how its player rolls (RN-18: "No app" or "Meus próprios dados"). */
@Component({
  selector: 'app-player-row',
  imports: [CombatantToken, MatIconModule],
  template: `
    <div class="row">
      <label class="check">
        <input type="checkbox" class="box" [checked]="included()" (change)="toggle.emit()" />
        <span class="mark" aria-hidden="true"><mat-icon>check</mat-icon></span>
        <span class="mr-visually-hidden">Incluir {{ name() }}</span>
      </label>
      <app-combatant-token [initial]="initial()" [size]="30" />
      <span class="text">
        <span class="name">{{ name() }}</span>
        <span class="sub">{{ sub() }}</span>
      </span>
      <span class="mr-tag">
        <mat-icon aria-hidden="true">{{ inApp() ? 'casino' : 'person' }}</mat-icon>{{ roll() }}
      </span>
    </div>
  `,
  styles: `
    :host {
      display: block;
      border-top: 1px solid var(--mr-rule);
    }

.row {
  display: grid;
  grid-template-columns: 24px 30px minmax(0, 1fr) auto;
  align-items: center;
  gap: 12px;
  min-height: 56px;
  padding: 6px 0;
}

.text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.name {
  font-size: 16px;
  font-weight: 700;
  line-height: 21px;
}

.sub {
  font-size: 14px;
  line-height: 19px;
  color: var(--mr-ink-muted);
}

// A 24px box in a 44px target; a real checkbox underneath, drawn over.
.check {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
}

.box {
  position: absolute;
  inset: -10px;
  margin: 0;
  opacity: 0;
  cursor: pointer;
}

.mark {
  display: flex;
  align-items: center;
  justify-content: center;
  box-sizing: border-box;
  width: 24px;
  height: 24px;
  border: 2px solid var(--mr-control-line);
  border-radius: 5px;
  color: transparent;
  pointer-events: none;

  .mat-icon {
    width: 18px;
    height: 18px;
    font-size: 18px;
  }
}

.box:checked + .mark {
  border-color: var(--mr-ink);
  background: var(--mr-ink);
  color: var(--mr-surface);
}

.box:focus-visible + .mark {
  outline: 2px solid var(--mr-focus);
  outline-offset: 2px;
}

  `,
})
export class PlayerRow {
  readonly name = input.required<string>();
  readonly sub = input('');
  readonly roll = input('');
  readonly inApp = input(true);
  readonly included = input(false);

  readonly toggle = output<void>();

  protected readonly initial = computed(() => combatantInitial(this.name()));
}
