import { Injectable, TemplateRef, signal } from '@angular/core';

/**
 * The hole the app bar leaves for the player's "Anotações" button (E8-06,
 * MR-030). The session page (a lazy chunk) hands over the button's template
 * and the bar only draws it, so the shell carries no notes code at all: the
 * initial bundle stays as small as before.
 */
@Injectable({ providedIn: 'root' })
export class SessionNotes {
  /** The button's template while a player's session page is open; `null` otherwise. */
  readonly bar = signal<TemplateRef<unknown> | null>(null);
}
