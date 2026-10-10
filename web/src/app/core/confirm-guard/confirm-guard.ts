import { DOCUMENT } from '@angular/common';
import { DestroyRef, Injectable, inject } from '@angular/core';

/**
 * How long a freshly opened inline confirmation ignores clicks. A double-click's
 * second click arrives within about 300 ms, so 500 ms keeps it from landing on
 * the destructive button that appeared under the pointer.
 */
export const CONFIRM_GUARD_MS = 500;

/**
 * The inline confirmations of the app: the container that replaces a button with
 * "Descartar o dano de 2? Voltar / Descartar" and its kin. A new one that does
 * not use one of these classes or roles marks its container `data-confirm`.
 */
export const CONFIRM_SELECTOR = '[role="alertdialog"], [data-confirm], .ask, .confirm, .question';

/**
 * The double-click guard of every inline confirmation (docs/design.md, "Confirm in
 * place"). While a confirmation container is younger than {@link CONFIRM_GUARD_MS},
 * every click on a button or link inside it is swallowed and its buttons carry `aria-disabled="true"`
 * (so Playwright's actionability check waits for them). Keyboard use is the same:
 * Enter on the focused button is a click. Started once, at app start.
 */
@Injectable({ providedIn: 'root' })
export class ConfirmGuard {
  private readonly doc = inject(DOCUMENT);
  private readonly openedAt = new WeakMap<Element, number>();
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
    if (this.openedAt.has(container)) {
      return;
    }
    this.openedAt.set(container, performance.now());
    const marked: Element[] = [];
    container.querySelectorAll('button').forEach((button) => {
      if (!button.hasAttribute('aria-disabled')) {
        button.setAttribute('aria-disabled', 'true');
        marked.push(button);
      }
    });
    setTimeout(
      () => marked.forEach((button) => button.removeAttribute('aria-disabled')),
      CONFIRM_GUARD_MS,
    );
  }

  private onClick(event: Event): void {
    const target = event.target;
    if (!(target instanceof Element)) {
      return;
    }
    // Only what acts: a field or a radio inside the container stays usable.
    if (!target.closest('button, a, [role="button"]')) {
      return;
    }
    const now = performance.now();
    let el = target.closest(CONFIRM_SELECTOR);
    while (el) {
      const opened = this.openedAt.get(el);
      if (opened !== undefined && now - opened < CONFIRM_GUARD_MS) {
        event.preventDefault();
        event.stopImmediatePropagation();
        return;
      }
      el = el.parentElement?.closest(CONFIRM_SELECTOR) ?? null;
    }
  }
}
