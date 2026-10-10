import { Code, ConnectError } from '@connectrpc/connect';
import { signal } from '@angular/core';

import { type ExtraDieField, genericDieFields, missingDice } from './effects';

/** The size of the dice the effects add to a roll for now (Bênção, Perdição, Orientação, Resistência: d4). */
const EXTRA_DIE_FACES = 4;

/**
 * The d4 an effect adds to a roll that is typed from physical dice, outside the attack: the app does not know which
 * effects are on the character (the master may keep some from the players), so the roll is sent and, when the server
 * says it takes more dice ("the roll takes N more die(s)"), the sheet asks for them and the player sends it again with
 * the faces. Held by the sheet that rolls; `ExtraDice` draws the fields. With the app's dice nothing is asked or sent.
 */
export class ExtraDiceState {
  /** The dice to ask for; empty until the server says the roll takes some. */
  readonly fields = signal<readonly ExtraDieField[]>([]);
  /** The faces typed, aligned with the fields (`null` where one is empty or out of range). */
  readonly faces = signal<readonly (number | null)[]>([]);

  /** Reads a refusal; when it is "the roll takes N more dice" the fields appear and the sentence to show is returned, else `''`. */
  fromRefusal(err: unknown): string {
    const e = ConnectError.from(err, Code.Unavailable);
    const more = e.code === Code.InvalidArgument ? missingDice(e.rawMessage) : 0;
    if (more === 0) {
      return '';
    }
    this.fields.set(genericDieFields(more, EXTRA_DIE_FACES));
    return more === 1
      ? 'Esta rolagem leva mais um d4: role-o e digite o resultado.'
      : `Esta rolagem leva mais ${more} d4: role-os e digite os resultados.`;
  }

  /** The faces to send with a roll: none for the app's dice or while no die was asked; `null` while one is missing. */
  take(typed: boolean): readonly number[] | null {
    const fields = this.fields();
    if (!typed || fields.length === 0) {
      return [];
    }
    const faces = this.faces();
    return faces.length !== fields.length || faces.some((f) => f === null)
      ? null
      : (faces as readonly number[]);
  }

  /** What to say when `take` found a die missing. */
  missingText(): string {
    const fields = this.fields();
    return fields.length === 1
      ? `Digite o resultado do d${fields[0].faces} antes de confirmar.`
      : 'Digite o resultado de cada dado extra antes de confirmar.';
  }
}
