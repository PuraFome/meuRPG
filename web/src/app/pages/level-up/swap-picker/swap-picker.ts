import { ChangeDetectionStrategy, Component, input } from '@angular/core';

import type { LevelUpSwap } from '../../../core/levelup/levelup-swap';

/**
 * "Trocar uma escolha" (MR-025): one panel, in the Ability Score Improvement step or, at a level without it, in the
 * "Escolhas" step. Its groups are the table rule's "Trocar uma escolha no aumento de atributo" (at an Ability Score
 * Improvement level, only where the table plays with it) and the table features that allow a swap at the level. Optional
 * ("Não trocar" is the start): a group, the pick to give up and the pick to take in its place; the choice of each group
 * stays when another is opened. The pool may hold options the sheet does not qualify for yet; the server judges the
 * prerequisite when the level is confirmed.
 */
@Component({
  selector: 'app-swap-picker',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="mr-panel" aria-labelledby="swap-title">
      <h2 class="mr-panel__title" id="swap-title">Trocar uma escolha</h2>
      <p class="lead">Neste nível você pode trocar uma escolha anterior por outra do mesmo conjunto. É opcional.</p>
      <div class="opts" role="radiogroup" aria-labelledby="swap-title">
        <label class="opt" [class.opt--on]="swap().groupKey() === ''">
          <input type="radio" name="swap-group" class="opt__input" [checked]="swap().groupKey() === ''" (change)="swap().clear()" />
          Não trocar
        </label>
        @for (g of swap().groups(); track g.group) {
          <label class="opt" [class.opt--on]="swap().groupKey() === g.group">
            <input type="radio" name="swap-group" class="opt__input" [checked]="swap().groupKey() === g.group" (change)="swap().setGroup(g.group)" />
            {{ g.namePt }}
            @if (swap().isWhole(g.group)) {
              <span class="opt__hidden"> (troca escolhida)</span>
            }
          </label>
        }
      </div>
      @if (swap().group(); as g) {
        <h3 class="sub" id="swap-from">Abrir mão de</h3>
        <div class="opts opts--list" role="radiogroup" aria-labelledby="swap-from">
          @for (p of g.picks; track p.key) {
            <label class="opt" [class.opt--on]="swap().fromKey() === p.key">
              <input type="radio" name="swap-from" class="opt__input" [checked]="swap().fromKey() === p.key" (change)="swap().setFrom(p.key)" />
              {{ p.namePt }}
            </label>
          }
        </div>
        <h3 class="sub" id="swap-to">Pegar no lugar</h3>
        <div class="opts opts--list" role="radiogroup" aria-labelledby="swap-to">
          @for (p of g.pool; track p.key) {
            <label class="opt" [class.opt--on]="swap().toKey() === p.key">
              <input type="radio" name="swap-to" class="opt__input" [checked]="swap().toKey() === p.key" (change)="swap().setTo(p.key)" />
              {{ p.namePt }}
            </label>
          }
        </div>
        @if (swap().summary(); as line) {
          <p class="line" role="status">{{ line }}</p>
        } @else {
          <p class="lead">Escolha o que deixar e o que pegar. Sem os dois, a troca não vale.</p>
        }
      }
    </section>
  `,
  styles: `
    .lead {
      margin: 0 0 var(--mr-space-3);
      font-size: var(--mr-font-15);
      line-height: 20px;
      color: var(--mr-ink-muted);
    }

    .sub {
      margin: var(--mr-space-3) 0 var(--mr-space-2);
      font-size: var(--mr-font-15);
      font-weight: 700;
    }

    .opts {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
      gap: var(--mr-space-2);

      &--list {
        grid-template-columns: 1fr;
      }
    }

    .opt {
      position: relative;
      display: flex;
      align-items: center;
      box-sizing: border-box;
      min-height: 44px;
      padding: 0 var(--mr-space-3);
      border: 1px solid var(--mr-control-line);
      border-radius: var(--mr-radius-sm);
      font-weight: 700;
      cursor: pointer;

      &--on::before {
        content: '✓';
        margin-right: var(--mr-space-2);
      }

      &--on {
        padding: 0 11px;
        border: 2px solid var(--mr-accent);
        background: var(--mr-accent-soft);
      }

      &:has(:focus-visible) {
        outline: 2px solid var(--mr-focus);
        outline-offset: 2px;
      }
    }

    .opt__hidden {
      position: absolute;
      width: 1px;
      height: 1px;
      overflow: hidden;
      clip-path: inset(50%);
      white-space: nowrap;
    }

    .opt__input {
      position: absolute;
      opacity: 0;
      width: 1px;
      height: 1px;
    }

    .line {
      margin: var(--mr-space-3) 0 0;
      font-weight: 700;
    }
  `,
})
export class SwapPicker {
  readonly swap = input.required<LevelUpSwap>();
}
