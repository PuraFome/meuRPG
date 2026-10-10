import { DOCUMENT } from '@angular/common';
import { DestroyRef, Injectable, inject } from '@angular/core';

/**
 * How long a freshly opened inline confirmation ignores clicks. A double-click's
 * second click arrives within about 300 ms, so 500 ms keeps it from landing on
 * the destructive button that appeared under the pointer.
 */
export const CONFIRM_GUARD_MS = 500;

/**
 * The confirmations the guard watches: an inline confirmation that replaces a
 * button with a destructive choice in the same spot ("Descartar o dano de 2?
 * Voltar / Descartar") marks its container `data-confirm`. Opt-in on purpose: a
 * guard on every form-like container swallowed ordinary clicks (a puzzle's
 * "Responder", the grid change's second step).
 */
export const CONFIRM_SELECTOR = '[data-confirm]';

/**
 * The double-click guard of the inline confirmations (docs/design.md, "Confirm in
 * place"). For {@link CONFIRM_GUARD_MS} after a `data-confirm` container appears,
 * its buttons carry `aria-disabled="true"` (their own value comes back after) and
 * a click on one of them is swallowed. Only a button the guard marked is guarded,
 * so what the screen says (disabled) and what a click does always agree, for a
 * person and for Playwright's actionability check. Started once, at app start.
 */
@Injectable({ providedIn: 'root' })
export class ConfirmGuard {
  private readonly doc = inject(DOCUMENT);
  private readonly armed = new WeakSet<Element>();
  /** Until when (performance.now()) a marked button ignores clicks. */
  private readonly guardedUntil = new WeakMap<Element, number>();
  private readonly destroyRef = inject(DestroyRef);
  private started = false;

  start(): void {
    const view = this.doc.defaultView;
    if (this.started || !view || typeof view.MutationObserver === 'undefined') {
      return;
    }
    this.started = true;
    const onClick = (event: Event) => this.onClick(event);
    this.doc.addEventListener('click', onClick, true);
    const observer = new view.MutationObserver((records) => {
      for (const record of records) {
        record.addedNodes.forEach((node) => this.onAdded(node));
      }
    });
    observer.observe(this.doc.documentElement, { childList: true, subtree: true });
    // The app never destroys its root injector; a test's TestBed does.
    this.destroyRef.onDestroy(() => {
      this.doc.removeEventListener('click', onClick, true);
      observer.disconnect();
    });
    this.doc.querySelectorAll(CONFIRM_SELECTOR).forEach((el) => this.arm(el));
  }

  private onAdded(node: Node): void {
    if (node.nodeType !== 1) {
      return;
    }
    const el = node as Element;
    if (el.matches(CONFIRM_SELECTOR)) {
      this.arm(el);
    }
    el.querySelectorAll(CONFIRM_SELECTOR).forEach((inner) => this.arm(inner));
  }

  private arm(container: Element): void {
    if (this.armed.has(container)) {
      return;
    }
    this.armed.add(container);
    const until = performance.now() + CONFIRM_GUARD_MS;
    const marked: [Element, string | null][] = [];
    container.querySelectorAll('button').forEach((button) => {
      marked.push([button, button.getAttribute('aria-disabled')]);
      button.setAttribute('aria-disabled', 'true');
      this.guardedUntil.set(button, until);
    });
    setTimeout(() => {
      for (const [button, own] of marked) {
        this.guardedUntil.delete(button);
        // Give the button its own value back, unless the app changed it meanwhile.
        if (button.getAttribute('aria-disabled') === 'true') {
          if (own === null) {
            button.removeAttribute('aria-disabled');
          } else {
            button.setAttribute('aria-disabled', own);
          }
        }
      }
    }, CONFIRM_GUARD_MS);
  }

  private onClick(event: Event): void {
    const target = event.target;
    if (!(target instanceof Element)) {
      return;
    }
    const button = target.closest('button');
    const until = button ? this.guardedUntil.get(button) : undefined;
    if (until !== undefined && performance.now() < until) {
      event.preventDefault();
      event.stopImmediatePropagation();
    }
  }
}
