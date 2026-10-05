/**
 * Moves the focus to an element that replaced the one the person used, and makes it show the focus
 * ring. A ring drawn only for `:focus-visible` is skipped by the browser when the focus moves by
 * code after a mouse click, which leaves a question asking "Voltar?" with nothing showing where the
 * focus is; the attribute draws it (`[data-ring]` in `_ui.scss`) until the element loses the focus.
 */
export function focusWithRing(el: HTMLElement | null | undefined): void {
  if (!el) {
    return;
  }
  el.focus({ preventScroll: true });
  el.setAttribute('data-ring', '');
  el.addEventListener('blur', () => el.removeAttribute('data-ring'), { once: true });
}
