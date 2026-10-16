import { Injector, afterNextRender } from '@angular/core';

/** A new stage starts at the top, with the title focused (the screen reader reads where it is). */
export function focusAfterStage(
  injector: Injector,
  scrollToTop: () => void,
  field?: 'npc' | 'refs',
): void {
  afterNextRender(
    () => {
      scrollToTop();
      // The field the server named, when it did, otherwise the title.
      const target = field ? document.querySelector<HTMLElement>(`[data-field="${field}"]`) : null;
      (target ?? document.getElementById('gen-t'))?.focus({ preventScroll: !target });
    },
    { injector },
  );
}

/** The question is at the end of the body, with its two buttons: it scrolls there. */
export function scrollToQuestion(injector: Injector): void {
  afterNextRender(
    () => {
      const title = document.getElementById('gen-ask-t');
      title?.scrollIntoView?.({ block: 'nearest' });
      title?.focus({ preventScroll: true });
    },
    { injector },
  );
}
