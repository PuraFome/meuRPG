import { ChangeDetectionStrategy, Component, ElementRef, viewChild } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { keepClear } from './scroll-clearance';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `<div #bar style="position: sticky; bottom: 20px; height: 40px"></div>`,
})
class Host {
  readonly bar = viewChild<ElementRef<HTMLElement>>('bar');
  constructor() {
    keepClear(() => this.bar()?.nativeElement, 'bottom');
  }
}

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `<div #bar style="height: 40px"></div>`,
})
class Plain {
  readonly bar = viewChild<ElementRef<HTMLElement>>('bar');
  constructor() {
    keepClear(() => this.bar()?.nativeElement, 'top');
  }
}

describe('keepClear', () => {
  const realResize = globalThis.ResizeObserver;
  const realHeight = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'offsetHeight')!;

  beforeEach(() => {
    globalThis.ResizeObserver = class {
      observe() {}
      disconnect() {}
      unobserve() {}
    } as never;
    // jsdom has no layout: the bar is 40 px tall.
    Object.defineProperty(HTMLElement.prototype, 'offsetHeight', {
      configurable: true,
      get: () => 40,
    });
  });

  afterEach(() => {
    globalThis.ResizeObserver = realResize;
    Object.defineProperty(HTMLElement.prototype, 'offsetHeight', realHeight);
    document.documentElement.style.scrollPaddingBottom = '';
    document.documentElement.style.scrollPaddingTop = '';
  });

  it('sets the page’s scroll padding to the bar, its offset and a gap while it sticks, and clears it after', async () => {
    const fixture = TestBed.createComponent(Host);
    fixture.detectChanges();
    await fixture.whenStable();
    expect(document.documentElement.style.scrollPaddingBottom).toBe('76px');
    fixture.destroy();
    expect(document.documentElement.style.scrollPaddingBottom).toBe('');
  });

  it('leaves the padding alone for a bar that does not stick', async () => {
    const fixture = TestBed.createComponent(Plain);
    fixture.detectChanges();
    await fixture.whenStable();
    expect(document.documentElement.style.scrollPaddingTop).toBe('');
  });
});
