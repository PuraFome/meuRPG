import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { latestInput } from './latest-input';

describe('latestInput', () => {
  function setup(first: string) {
    const input = signal(first);
    const latest = TestBed.runInInjectionContext(() => latestInput(input));
    TestBed.tick();
    return { input, latest };
  }

  it('reads the input until this component sends something', () => {
    const { latest } = setup('a');
    expect(latest.current()).toBe('a');
  });

  it('builds the second edit before the parent renders on the first one, not on the old input', () => {
    const { latest } = setup('a');
    latest.sent('a+pick');
    expect(latest.current()).toBe('a+pick');
    latest.sent(`${latest.current()}+field`);
    expect(latest.current()).toBe('a+pick+field');
  });

  it('takes a genuinely different entity when the input brings one, and the next edit builds on it', () => {
    const { input, latest } = setup('a');
    latest.sent('a+edit');
    // The editor loads another draft (setInput with another entity): what was sent no longer counts.
    input.set('other');
    TestBed.tick();
    expect(latest.current()).toBe('other');
    latest.sent(`${latest.current()}+edit`);
    expect(latest.current()).toBe('other+edit');
  });

  it('follows the parent when it renders what this component sent', () => {
    const { input, latest } = setup('a');
    latest.sent('b');
    input.set('b');
    TestBed.tick();
    expect(latest.current()).toBe('b');
  });
});
