import { Injectable, signal } from '@angular/core';

/**
 * Whether the dice make a sound: off until the player switches it on with "Som dos dados", and only for the open tab.
 * It is kept in memory, in this signal, and nowhere else: not in web storage and not in a cookie (docs/privacy.md), so
 * a new tab or a reload starts silent again.
 */
@Injectable({ providedIn: 'root' })
export class DiceSoundSetting {
  private readonly state = signal(false);

  readonly on = this.state.asReadonly();

  /** Flips the setting and returns the new value. */
  toggle(): boolean {
    this.state.update((on) => !on);
    return this.state();
  }
}
