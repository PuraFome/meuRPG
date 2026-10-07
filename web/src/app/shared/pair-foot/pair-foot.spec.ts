import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { needsStack } from './pair-foot';

/** A pair of Material-like buttons with fixed word widths, in a room of a fixed width, on a computer or a phone. */
function pair(room: number, words: [number, number], desktop: boolean): HTMLElement {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: desktop && query.includes('768'),
    media: query,
  }));
  const parent = document.createElement('div');
  Object.defineProperty(parent, 'clientWidth', { value: room });
  const foot = document.createElement('div');
  foot.style.columnGap = '12px';
  Object.defineProperty(foot, 'clientWidth', { value: room });
  parent.append(foot);
  for (const width of words) {
    const button = document.createElement('button');
    button.style.paddingLeft = '16px';
    button.style.paddingRight = '16px';
    const label = document.createElement('span');
    label.className = 'mdc-button__label';
    label.getBoundingClientRect = () => ({ width }) as DOMRect;
    // The parts of a Material button that are as wide as the button: they must not count.
    for (const cls of [
      'mat-mdc-button-persistent-ripple',
      'mat-focus-indicator',
      'mat-mdc-button-touch-target',
    ]) {
      const part = document.createElement('span');
      part.className = cls;
      part.getBoundingClientRect = () => ({ width: room }) as DOMRect;
      button.append(part);
    }
    button.append(label);
    foot.append(button);
  }
  document.body.append(parent);
  return foot;
}

describe('needsStack', () => {
  beforeEach(() => undefined);
  afterEach(() => {
    vi.unstubAllGlobals();
    document.body.innerHTML = '';
  });

  it('keeps the pair side by side on a computer, in a normal sheet', () => {
    expect(needsStack(pair(480, [60, 120], true))).toBe(false);
  });

  it('stacks it in a 380 px column, where two equal buttons do not fit', () => {
    expect(needsStack(pair(330, [60, 120], true))).toBe(true);
  });

  it('on a phone, keeps short words side by side and stacks when one label needs more than half', () => {
    expect(needsStack(pair(358, [50, 90], false))).toBe(false);
    expect(needsStack(pair(358, [90, 200], false))).toBe(true);
  });

  it('measures the words and the icon, never the ripple or the touch target that are as wide as the button', () => {
    // If those parts counted, even a short pair would "need" the whole room and stack on every computer screen.
    expect(needsStack(pair(1280, [40, 40], true))).toBe(false);
    expect(needsStack(pair(358, [40, 40], false))).toBe(false);
  });
});
