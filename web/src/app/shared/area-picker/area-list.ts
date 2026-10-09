import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { AllyWarning, AreaRow } from '../../core/combat/area-text';
import { combatantInitial } from '../../core/combat/combat-view';
import { CombatantToken } from '../combatant-token/combatant-token';

/**
 * Step 2 of an area spell, "Quem está na área" (PM-02a state 3, PM-02b states 4 and 6, PM-02d state 11): who the server
 * says is inside, each with the state word (never hit points, RN-20) and the cover against the point of origin as text,
 * the ally warning (`role="alert"`) and the count. A player's list holds only what they see (RN-10): nothing here counts
 * or hints at anything else. With nobody inside, the list gives way to the confirmation in place ("Ninguém que você vê
 * está na área."), and the page's buttons become "Conjurar mesmo assim" and "Mudar o local".
 */
@Component({
  selector: 'app-area-list',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CombatantToken, MatIconModule],
  template: `
    @if (nobody(); as n) {
      <div class="mr-notice mr-notice--warning" role="alert">
        <mat-icon aria-hidden="true">warning</mat-icon>
        <p><strong>{{ n.strong }}</strong> {{ n.rest }}</p>
      </div>
    } @else {
      @if (warning(); as w) {
        <div class="mr-notice mr-notice--warning" role="alert">
          <mat-icon aria-hidden="true">warning</mat-icon>
          <p><strong>{{ w.strong }}</strong> {{ w.rest }}</p>
        </div>
      }
      <div class="head">
        <h4 class="head__title">{{ heading() }}</h4>
        <span class="head__count">{{ count() }}</span>
      </div>
      <ul class="rows" [attr.aria-label]="heading()">
        @for (r of rows(); track r.id) {
          <li class="row" [class.row--hidden]="r.hidden">
            <app-combatant-token [initial]="initial(r.label)" [npc]="npcs().has(r.id)" [creature]="creatures().has(r.id)" [hidden]="r.hidden" [size]="34" />
            <span class="row__text">
              <span class="row__top">
                <b class="row__name">{{ r.label }}</b>
                @if (r.ally) {
                  <span class="mr-tag mr-tag--pending row__tag"><mat-icon aria-hidden="true">warning</mat-icon>Aliado</span>
                }
                @if (r.hidden) {
                  <span class="mr-tag row__tag"><mat-icon aria-hidden="true">visibility_off</mat-icon>Escondido</span>
                }
                @if (r.state) {
                  <span class="row__state">{{ r.state }}</span>
                }
              </span>
              <span class="row__cover">
                @if (r.mark) {
                  <span class="mr-swatch mr-swatch--sm" [class.mr-swatch--half]="r.mark === 'half'" [class.mr-swatch--three]="r.mark === 'three'" aria-hidden="true"></span>
                }
                <span>{{ r.cover }}@if (r.distance) { · {{ r.distance }}}</span>
              </span>
            </span>
          </li>
        }
      </ul>
      @if (note()) {
        <p class="note">{{ note() }}</p>
      }
    }
    <p class="mr-visually-hidden" role="status">{{ summary() }}</p>
  `,
  styleUrl: './area-list.scss',
})
export class AreaList {
  readonly rows = input.required<readonly AreaRow[]>();
  /** "4 criaturas", or the master's "4 criaturas · 1 escondida". */
  readonly count = input('');
  /** "Quem você vê na área" for a player, "Na área" for the master. */
  readonly heading = input('Quem você vê na área');
  readonly warning = input<AllyWarning | null>(null);
  /** The confirmation in place of the list, when nobody the caster sees is inside. */
  readonly nobody = input<{ strong: string; rest: string } | null>(null);
  /** The line under the list (how the cover is measured), or `''`. */
  readonly note = input('');
  /** The ids drawn as an NPC's square and as a player's creature. */
  readonly npcs = input<ReadonlySet<string>>(new Set());
  readonly creatures = input<ReadonlySet<string>>(new Set());

  /** What the live region says when the step opens: "4 criaturas na área; Toren é aliado". */
  protected readonly summary = computed(() => {
    if (this.nobody()) {
      return '';
    }
    const n = this.rows().length;
    const allies = this.rows().filter((r) => r.ally);
    const base = `${n} ${n === 1 ? 'criatura' : 'criaturas'} na área`;
    if (allies.length === 0) {
      return base;
    }
    const names = allies.map((r) => r.label);
    return `${base}; ${names.join(', ')} ${names.length === 1 ? 'é aliado' : 'são aliados'}`;
  });

  protected initial(label: string): string {
    return combatantInitial(label.replace(/ \(você\)$/, ''));
  }
}
