import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { Ability, type FeatOption } from '../../../gen/meurpg/rules/v1/rules_pb';
import {
  type KeyName,
  abilityName,
  prerequisiteSupport,
  unmetText,
} from '../../core/content/feat-text';

/** One feat of the list, with what the card says about it. */
interface FeatCard {
  readonly feat: FeatOption;
  readonly picked: boolean;
  /** "Força 13, nível 4 ou mais"; empty when the feat asks nothing. */
  readonly asks: string;
  /** Why a feat the character does not qualify for is off, in sentences. */
  readonly why: readonly string[];
  /** "Escolha 1 entre Força e Destreza, +1 em cada." */
  readonly increase: string;
  /** The abilities to tick, when the player has a choice to make (the feat lists more than it raises). */
  readonly choices: readonly { ability: Ability; label: string; on: boolean; off: boolean }[];
}

/**
 * The feat picker (MR-025): the feats the server offers a character, each with its text, what it asks and, for the ones the
 * character does not qualify for, why (shown but off). A feat that raises abilities lets the player tick the abilities it
 * asks for, from its own list. Whether the character qualifies and whether an ability can still rise are the server's: this
 * only shows `qualifies` and `unmet`, and the parent asks the server again with the picks. Used by the guided level-up.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-feat-picker',
  imports: [MatIconModule],
  template: `
    <fieldset class="picker">
      <legend class="mr-visually-hidden">{{ label() }}</legend>
      <ul class="cards">
        @for (c of cards(); track c.feat.key) {
          <li>
            <div class="card" [class.card--on]="c.picked" [class.card--off]="!c.feat.qualifies">
              <label class="card__head">
                <input
                  type="radio"
                  class="card__input"
                  [name]="name()"
                  [checked]="c.picked"
                  [disabled]="!c.feat.qualifies"
                  (change)="picked.emit(c.feat.key)"
                />
                <span class="card__mark" aria-hidden="true"></span>
                <span class="card__name">{{ c.feat.namePt }}</span>
                @if (c.feat.table) {
                  <span class="mr-tag"><mat-icon aria-hidden="true">menu_book</mat-icon>Da mesa</span>
                }
              </label>
              <p class="card__line">{{ c.asks ? 'Pede: ' + c.asks + '.' : 'Não pede nada.' }}</p>
              @if (c.increase) {
                <p class="card__line">{{ c.increase }}</p>
              }
              @if (c.why.length > 0) {
                <ul class="card__why">
                  @for (w of c.why; track w) {
                    <li><mat-icon aria-hidden="true">block</mat-icon>{{ w }}</li>
                  }
                </ul>
              }
              @if (c.feat.desc.length > 0) {
                <details class="card__text">
                  <summary>Ler o talento</summary>
                  @for (p of c.feat.desc; track $index) {
                    <p>{{ p }}</p>
                  }
                  @if (!c.feat.table) {
                    <p class="card__src">Texto do SRD, em inglês.</p>
                  }
                </details>
              }
              @if (c.picked && c.choices.length > 0) {
                <div class="abilities" role="group" [attr.aria-label]="'Habilidades de ' + c.feat.namePt">
                  @for (a of c.choices; track a.ability) {
                    <label class="ability" [class.ability--on]="a.on">
                      <input type="checkbox" class="ability__input" [checked]="a.on" [disabled]="a.off" (change)="abilityToggled.emit(a.ability)" />
                      <span class="ability__box" aria-hidden="true">
                        @if (a.on) {
                          <mat-icon>check</mat-icon>
                        }
                      </span>
                      {{ a.label }}
                    </label>
                  }
                </div>
              }
            </div>
          </li>
        }
      </ul>
    </fieldset>
  `,
  styleUrl: './feat-picker.scss',
})
export class FeatPicker {
  readonly feats = input.required<readonly FeatOption[]>();
  /** The key of the picked feat, empty for none. */
  readonly selected = input('');
  /** The abilities ticked for the picked feat. */
  readonly abilities = input<readonly Ability[]>([]);
  /** Names a proficiency or a race key of a prerequisite. */
  readonly nameOf = input<KeyName>((key) => key.replace(/^[a-z-]+:/, ''));
  readonly label = input('Talento');
  /** The radio group's name: one per page. */
  readonly name = input('feat');

  readonly picked = output<string>();
  readonly abilityToggled = output<Ability>();

  protected readonly cards = computed<FeatCard[]>(() =>
    this.feats().map((feat) => {
      const picked = feat.key === this.selected();
      const inc = feat.increase;
      const full = this.abilities().length >= (inc?.count ?? 0);
      return {
        feat,
        picked,
        asks: prerequisiteSupport(feat.prerequisite, this.nameOf()),
        why: feat.qualifies ? [] : feat.unmet.map((u) => unmetText(u, this.nameOf())),
        increase: inc ? increaseText(inc.count, inc.from, inc.value) : '',
        // A feat that raises all the abilities it lists leaves nothing to tick.
        choices:
          inc && inc.count < inc.from.length
            ? inc.from.map((ability) => {
                const on = this.abilities().includes(ability);
                return {
                  ability,
                  label: abilityName(ability),
                  on,
                  off: !on && inc.count > 1 && full,
                };
              })
            : [],
      };
    }),
  );
}

/** "Aumenta Força e Destreza em +1." / "Escolha 1 entre Força e Destreza, +1 em cada." */
function increaseText(count: number, from: readonly Ability[], value: number): string {
  const names = new Intl.ListFormat('pt-BR', { type: 'conjunction' }).format(from.map(abilityName));
  if (count >= from.length) {
    return `Aumenta ${names} em +${value}.`;
  }
  return count === 1
    ? `Escolha 1 habilidade entre ${names}: +${value}.`
    : `Escolha ${count} habilidades entre ${names}: +${value} em cada.`;
}
