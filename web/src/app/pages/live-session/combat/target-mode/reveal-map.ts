import { type Injector, afterNextRender } from '@angular/core';

/**
 * Brings the battle map into view once the page has drawn its highlight (C7-04, C7-05): on a phone the map is at the top of a long
 * page, so it scrolls to its top (under the page's own bars: they set the scroll padding); from 1024px the map is beside the
 * rail, so the page moves only as far as it must. With `prefers-reduced-motion` the scroll is a jump, not a glide.
 */
export function revealMap(host: HTMLElement, injector: Injector, laptop: boolean): void {
  afterNextRender(
    () => {
      const reduce =
        typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;
      const card = host.querySelector('app-combat-map-card');
      if (card instanceof HTMLElement && !laptop) {
        // The app bar sticks to the top of a phone's screen: the map's top (and the chips above its tokens) stop below it.
        card.style.scrollMarginTop = `${stuckTop()}px`;
      }
      card?.scrollIntoView?.({
        block: laptop ? 'nearest' : 'start',
        behavior: reduce ? 'auto' : 'smooth',
      });
    },
    { injector },
  );
}

/** The room the page's bar keeps at the top of the screen, and a gap under it, in px. */
const TOP_GAP = 8;

function stuckTop(): number {
  const bar = document.querySelector('.app-bar');
  return bar instanceof HTMLElement && getComputedStyle(bar).position === 'sticky'
    ? bar.offsetHeight + TOP_GAP
    : 0;
}

/** The button the player pressed to start the choice, where the focus goes back to; `null` when nothing in the page has it. */
export function pressedButton(host: HTMLElement): HTMLElement | null {
  const active = document.activeElement;
  return active instanceof HTMLElement && active !== document.body && host.contains(active)
    ? active
    : null;
}
