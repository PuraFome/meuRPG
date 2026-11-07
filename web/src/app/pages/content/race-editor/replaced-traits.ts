import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { TraitRow } from '../../../core/content/subrace-traits';
import { CheckRow } from '../../../shared/form-fields/check-row';

/**
 * "Traços de Corujeiro" on the subrace editor (MR-025, RN-23): the parent race's traits as a checklist, the marked ones replaced
 * by the subrace. With no race chosen yet it is locked, saying why. A marked key that is no longer a trait of the race (a pack
 * changed it) stays as a red row that says so, until it is unmarked. The page owns the keys; this draws them.
 */
@Component({
  selector: 'app-replaced-traits',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CheckRow, MatIconModule],
  template: `
    <section class="mr-panel block" aria-labelledby="replaced-t">
      <h2 class="mr-panel__title" id="replaced-t">{{ locked() ? 'Traços da raça' : 'Traços de ' + raceName() }}</h2>
      @if (locked()) {
        <p class="hint"><mat-icon aria-hidden="true">lock</mat-icon>Escolha a raça da sub-raça primeiro: aqui aparecem os traços dela para marcar.</p>
      } @else {
        @if (unknownRace()) {
          <p class="hint">Os traços das raças do SRD ainda não aparecem aqui. O que já estava marcado fica como está.</p>
        } @else {
          <p class="hint">Marque os traços da raça que esta sub-raça substitui. Quem é desta sub-raça não tem o traço marcado, nem a escolha e as perícias que só ele dá.</p>
        }
        @if (rows().length === 0 && !unknownRace()) {
          <p class="hint">{{ raceName() }} ainda não tem traços salvos para marcar.</p>
        }
        <ul class="rows">
          @for (r of rows(); track r.key) {
            <li class="rows__item">
              <app-check-row
                [label]="r.stale ? 'Um traço que não é de ' + raceName() : r.name"
                [sub]="r.stale ? 'Veio marcado num pacote' : r.summary"
                [checked]="r.checked"
                [invalid]="r.stale"
                [path]="pathOf(r.key)"
                (toggled)="toggled.emit({ key: r.key, on: $event })"
              >
                @if (r.checked && !r.stale) {
                  <span class="mr-tag"><mat-icon aria-hidden="true">swap_horiz</mat-icon>Substituído</span>
                }
              </app-check-row>
              @if (r.stale) {
                <p class="inline-issue" role="note"><mat-icon aria-hidden="true">warning</mat-icon>Este traço não é de {{ raceName() }}. Desmarque para salvar.</p>
              }
            </li>
          }
        </ul>
      }
    </section>
  `,
  styles: `
    :host {
      display: block;
    }

    .block {
      display: flex;
      flex-direction: column;
      gap: var(--mr-space-3);
      padding: var(--mr-space-5);

      .mr-panel__title {
        margin: 0;
      }
    }

    .hint {
      display: flex;
      align-items: flex-start;
      gap: 6px;
      margin: 0;
      font-size: var(--mr-font-15);
      line-height: 20px;
      color: var(--mr-ink-muted);

      .mat-icon {
        flex: none;
        width: 18px;
        height: 18px;
        font-size: var(--mr-font-18);
        margin-top: 1px;
      }
    }

    .rows {
      margin: 0;
      padding: 0;
      list-style: none;
    }

    .rows__item {
      padding: 4px 0;
      border-top: 1px solid var(--mr-rule);

      &:first-child {
        border-top: 0;
      }

      .mr-tag {
        margin-left: auto;
      }
    }

    .inline-issue {
      display: flex;
      align-items: flex-start;
      gap: 6px;
      margin: 0 0 var(--mr-space-2) 36px;
      font-size: var(--mr-font-15);
      color: var(--mr-danger-ink);

      .mat-icon {
        flex: none;
        width: 18px;
        height: 18px;
        font-size: var(--mr-font-18);
      }
    }
  `,
})
export class ReplacedTraits {
  readonly raceName = input('');
  readonly rows = input.required<readonly TraitRow[]>();
  /** The race is not chosen yet. */
  readonly locked = input(false);
  /** The race has no trait list the page can read (an SRD race): the keys already marked are kept untouched. */
  readonly unknownRace = input(false);
  /** The path of the subrace's `replaces_trait_keys`, for a refusal to find its row. */
  readonly basePath = input('table_subrace.replaces_trait_keys');
  /** The place of each marked key in the list, by key: the index a refusal names. */
  readonly places = input<ReadonlyMap<string, number>>(new Map());
  readonly toggled = output<{ readonly key: string; readonly on: boolean }>();

  protected pathOf(key: string): string {
    const at = this.places().get(key);
    return at === undefined ? '' : `${this.basePath()}[${at}]`;
  }
}
