import { Component, computed, input } from '@angular/core';

import type { Creature } from '../../../gen/meurpg/rules/v1/rules_pb';
import { signed } from '../../core/creatures/creature-format';
import { type StatBlockHp, groupsOf, linesOf, tilesOf } from './stat-block-view';

export type { StatBlockHp } from './stat-block-view';

/**
 * A creature's stat block as a sheet (E9-10, quadro 3), from `GetCreature`:
 * armor class and hit points, the speeds, the six abilities, the lines of
 * saves, skills, senses, languages and challenge, then the book's traits and
 * actions. The labels are ours, in Portuguese; the names and the text of the
 * traits and actions are the SRD's English, marked once ("Os textos abaixo são
 * do livro de regras (SRD 5.1), em inglês.") and carrying `lang="en"` for a
 * screen reader. The browser computes nothing: every number is what the
 * server sent.
 */
@Component({
  selector: 'app-stat-block',
  template: `
    @let c = creature();
    <section class="sb mr-panel" [class.sb--full]="full()" aria-label="Ficha da criatura">
      <div class="tiles">
        @for (t of tiles(); track t.label) {
          <div class="tile">
            <span class="tile__l">{{ t.label }}</span>
            <span class="tile__v">{{ t.value }}@if (t.of) {&nbsp;<small>{{ t.of }}</small>}</span>
            @if (t.note) {
              <span class="tile__n">{{ t.note }}</span>
            }
          </div>
        }
      </div>

      <ul class="abilities" aria-label="Habilidades">
        @for (a of c.abilities; track a.ability) {
          <li class="ability">
            <span class="ability__n">{{ a.namePt }}</span>
            <b class="ability__m">{{ signed(a.modifier) }}</b>
            <span class="ability__s"><span class="mr-visually-hidden">valor </span>{{ a.score }}</span>
          </li>
        }
      </ul>

      <dl class="lines">
        @for (l of lines(); track l.term) {
          <div class="line">
            <dt>{{ l.term }}</dt>
            <dd>
              @for (p of l.parts; track $index) {
                @if (p.en) {
                  <span lang="en">{{ p.text }}</span>
                } @else {
                  {{ p.text }}
                }
              }
            </dd>
          </div>
        }
      </dl>

      @if (groups().length > 0) {
        <p class="srd">Os textos abaixo são do livro de regras (SRD 5.1), em inglês.</p>
      }
      @for (g of groups(); track g.title) {
        <section class="group" [attr.aria-label]="g.title || 'Características'">
          @if (g.title) {
            <h2 class="group__t">{{ g.title }}</h2>
          }
          @for (e of g.entries; track $index) {
            <div class="entry" lang="en">
              <h3 class="entry__n">{{ e.name }}@if (e.usage) { <span class="entry__u">({{ e.usage }})</span> }</h3>
              <p class="entry__t">{{ e.text }}</p>
            </div>
          }
        </section>
      }
      <ng-content />
    </section>
  `,
  styleUrl: './stat-block.scss',
})
export class StatBlock {
  readonly creature = input.required<Creature>();
  /** The creature's own hit points; the book's (the average) when unset. */
  readonly hp = input<StatBlockHp | null>(null);
  /**
   * The bestiary's page (MR-042, E10-08): each tile says where its number comes from (the armor, the
   * hit dice, the speed in feet), the challenge rating and its XP get a tile, and the proficiency
   * bonus a line. The creature's own page leaves it off.
   */
  readonly full = input(false);

  protected readonly signed = signed;

  protected readonly tiles = computed(() => tilesOf(this.creature(), this.hp(), this.full()));
  protected readonly lines = computed(() => linesOf(this.creature(), this.full()));
  protected readonly groups = computed(() => groupsOf(this.creature()));
}
