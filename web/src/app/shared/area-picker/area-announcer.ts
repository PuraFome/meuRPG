import { signal } from '@angular/core';

/** The pause before a move is announced, so holding an arrow key does not flood the screen reader. */
const ANNOUNCE_DELAY_MS = 300;

/** What the live region of the area map says: a move after a pause, a refusal at once. */
export class AreaAnnouncer {
  readonly spoken = signal('');
  private timer: ReturnType<typeof setTimeout> | null = null;

  /** Says `text` once the keys have stopped for a moment; a newer one replaces it. */
  say(text: string): void {
    this.clear();
    this.timer = setTimeout(() => this.spoken.set(text), ANNOUNCE_DELAY_MS);
  }

  now(text: string): void {
    this.clear();
    this.spoken.set(text);
  }

  clear(): void {
    if (this.timer !== null) {
      clearTimeout(this.timer);
      this.timer = null;
    }
  }
}
