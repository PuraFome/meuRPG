import { DestroyRef } from '@angular/core';

import { pageVisible } from './page-visible';

describe('pageVisible', () => {
  function setup(state: DocumentVisibilityState) {
    const target = new EventTarget();
    let current = state;
    Object.defineProperty(target, 'visibilityState', { get: () => current });
    const doc = target as unknown as Document;
    const callbacks: (() => void)[] = [];
    const destroyRef = {
      onDestroy: (fn: () => void) => callbacks.push(fn),
    } as unknown as DestroyRef;
    const visible = pageVisible(doc, destroyRef);
    return {
      visible,
      change: (next: DocumentVisibilityState) => {
        current = next;
        target.dispatchEvent(new Event('visibilitychange'));
      },
      destroy: () => callbacks.forEach((fn) => fn()),
    };
  }

  it('follows the browser tab', () => {
    const { visible, change } = setup('visible');
    expect(visible()).toBe(true);
    change('hidden');
    expect(visible()).toBe(false);
    change('visible');
    expect(visible()).toBe(true);
  });

  it('stops listening when the page is destroyed', () => {
    const { visible, change, destroy } = setup('visible');
    destroy();
    change('hidden');
    expect(visible()).toBe(true);
  });
});
