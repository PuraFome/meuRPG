import {
  DestroyRef,
  Directive,
  ElementRef,
  Injector,
  afterNextRender,
  effect,
  inject,
  input,
  output,
} from '@angular/core';

/**
 * Draws only the children of a list that fit in its height, so a TV never shows a row cut in half and never scrolls.
 * The host must have a height of its own (a flex or grid cell with `min-height: 0` and `overflow: hidden`); where it
 * has none (a phone, one column of natural height) every child fits and nothing is hidden. A child with a
 * `data-count` attribute is one the `fitted` output counts when it is left out, for the "+N".
 *
 * `appTvFit` takes whatever changes with the content (the list the template draws): the fit is made again after the
 * next render, and again whenever the host's size changes.
 */
@Directive({ selector: '[appTvFit]' })
export class TvFit {
  readonly content = input<unknown>(null, { alias: 'appTvFit' });
  /** How many counted children are left out. */
  readonly fitted = output<number>();

  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef).nativeElement;
  private readonly injector = inject(Injector);
  private last = -1;

  constructor() {
    effect(() => {
      this.content();
      afterNextRender(() => this.fit(), { injector: this.injector });
    });
    if (typeof ResizeObserver !== 'undefined') {
      const observer = new ResizeObserver(() => this.fit());
      observer.observe(this.host);
      inject(DestroyRef).onDestroy(() => observer.disconnect());
    }
  }

  /** Shows every child, then hides, from the first one that goes past the host's bottom edge, that one and the rest. */
  fit(): void {
    const children = [...this.host.children] as HTMLElement[];
    children.forEach((child) => child.style.removeProperty('display'));
    const limit = this.host.getBoundingClientRect().bottom;
    let left = 0;
    if (this.host.clientHeight > 0) {
      let past = false;
      for (const child of children) {
        past = past || child.getBoundingClientRect().bottom > limit + 1;
        if (past) {
          child.style.display = 'none';
          left += child.dataset['count'] === undefined ? 0 : 1;
        }
      }
    }
    if (left !== this.last) {
      this.last = left;
      this.fitted.emit(left);
    }
  }
}
