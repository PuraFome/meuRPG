import { type EffectRef, type ElementRef, effect } from '@angular/core';

/**
 * Takes the focus to the safe control of a release whose window went away ("Voltar à sessão"), so it does not fall to the page
 * behind: the control is drawn only in that state, and the focus moves as it appears. Build it in an injection context.
 */
export function focusSafeWhenGone(safe: () => ElementRef<HTMLElement> | undefined): EffectRef {
  return effect(() => safe()?.nativeElement.focus());
}
