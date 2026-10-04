import { Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { type PoolCard, article } from '../../../../core/combat/combat-log';

/**
 * What the master reads under a Sono or a Borrifo de Cores in the log (E8-03):
 * the dice and the total they make (the pool of hit points), then each creature
 * of the area from the lowest hit points up, with its hit points now, the
 * account of what is left of the total and the result in a word and an icon (the
 * colour carries nothing). Under it, once, how the spell works, who paid the slot
 * and "Mudar as condições" of each creature that got one, because the app does
 * not know a creature's type (the master changes the condition as always,
 * RN-02). Only the master gets any of this from the server; a player's log has
 * the sentence alone. On a laptop it is a table; on a phone, a card for each
 * creature.
 */
@Component({
  selector: 'app-pool-card',
  imports: [MatIconModule],
  template: `
    <section class="card" role="group" [attr.aria-label]="card().summary">
      <p class="card__cap">Dados rolados: total de PV que a magia afeta</p>
      <div class="card__top">
        <p class="card__roll">
          <span class="card__nb">{{ card().roll }}&nbsp;<span class="card__unit">PV</span></span>
          @if (card().physical) {
            {{ ' ' }}<span class="card__dice">dado físico</span>
          }
        </p>
        <span class="secret"><mat-icon aria-hidden="true">visibility_off</mat-icon>Só o mestre vê este quadro</span>
      </div>
      <p class="card__head" aria-hidden="true">
        <span>Na área, do menor PV ao maior</span><span>PV agora</span><span>Conta</span><span>Resultado</span>
      </p>
      <ul class="rows">
        @for (r of card().rows; track r.id) {
          <li class="row">
            <b class="row__name">{{ r.label }}</b>
            <span class="row__hp">{{ r.hitPoints }}</span>
            <span class="row__math">{{ r.math }}</span>
            <span class="row__word" [class.row__word--off]="!r.affected">
              <mat-icon aria-hidden="true">{{ r.icon }}</mat-icon>{{ r.word }}
            </span>
          </li>
        }
      </ul>
      <p class="card__note">{{ card().note }}</p>
      <div class="card__foot">
        <span class="card__slot">{{ card().slot }}</span>
        @for (c of card().changeFor; track c.id) {
          <button type="button" class="change" (click)="change.emit(c.id)">Mudar as condições {{ of(c.label) }} {{ c.label }}</button>
        }
      </div>
    </section>
  `,
  styleUrl: './pool-card.scss',
})
export class PoolCardView {
  readonly card = input.required<PoolCard>();
  /** "Mudar as condições": the creature whose conditions the master opens. */
  readonly change = output<string>();

  /** "do" or "da" before a name. */
  protected of(label: string): string {
    return article(label) === 'a' ? 'da' : 'do';
  }
}
