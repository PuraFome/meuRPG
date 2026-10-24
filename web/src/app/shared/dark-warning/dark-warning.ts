import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { type DarkCharacter, darkNames, darkNotice } from '../../core/maps/in-the-dark';

/**
 * The master's light warning (B3): the amber notice of the map card for the characters who see nothing where they
 * stand (no light they can see by, no darkvision that covers the square). One character: the sentence with "Ver no
 * mapa" and "Acender luz". Several: the count, the cause once, and a pair of buttons for each. `compact` (the map
 * page's painting mode) is one line of names with one "Ver no mapa". With nobody in the dark nothing is drawn: never an
 * empty notice. The status message under it ("A tocha de bilbo apagou. ...") sits in a polite live region that is on the
 * page before the message arrives, so it is announced. It only shows what it is given: the master's page decides
 * (RN-10: a player's page never has it), and `canLight` is whether the page has a quick light control to offer.
 */
@Component({
  selector: 'app-dark-warning',
  imports: [MatButtonModule, MatIconModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './dark-warning.html',
  styleUrl: './dark-warning.scss',
})
export class DarkWarning {
  readonly characters = input.required<readonly DarkCharacter[]>();
  /** The status message about a light that went out or was lit; empty for none. */
  readonly announcement = input('');
  readonly compact = input(false);
  /** Whether "Acender luz" is offered (the page has the carried-light control). */
  readonly canLight = input(true);

  /** "Ver no mapa": the page centers the map on this character. */
  readonly show = output<string>();
  /** "Acender luz": the page opens the quick light for this character. */
  readonly light = output<string>();

  protected readonly notice = computed(() => darkNotice(this.characters()));
  protected readonly names = computed(() => darkNames(this.characters()));
  protected readonly first = computed(() => this.characters()[0] ?? null);
}
