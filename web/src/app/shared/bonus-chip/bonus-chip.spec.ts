import { ChangeDetectionStrategy, Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { BonusChip } from './bonus-chip';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [BonusChip],
  template: `<app-bonus-chip [bonus]="bonus()" [counts]="counts()" [words]="words()" />`,
})
class Host {
  readonly bonus = signal(1);
  readonly counts = signal(true);
  readonly words = signal(false);
}

function setup() {
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  return { fixture, el, host: fixture.componentInstance };
}

describe('BonusChip', () => {
  it('is an image named with the bonus and that it is added', () => {
    const { el } = setup();
    const chip = el.querySelector('.bonus-chip')!;
    expect(chip.getAttribute('role')).toBe('img');
    expect(chip.getAttribute('aria-label')).toBe('Bônus mágico +1, já somado');
    expect(chip.textContent).toContain('auto_awesome');
    expect(chip.textContent).toContain('+1');
    expect(chip.classList).not.toContain('bonus-chip--idle');
  });

  it('says it does not count now, and is dashed', () => {
    const { fixture, el, host } = setup();
    host.bonus.set(3);
    host.counts.set(false);
    fixture.detectChanges();
    const chip = el.querySelector('.bonus-chip')!;
    expect(chip.getAttribute('aria-label')).toBe('Bônus mágico +3, não conta agora');
    expect(chip.classList).toContain('bonus-chip--idle');
  });

  it('shows the words after the chip, not hidden, when asked', () => {
    const { fixture, el, host } = setup();
    host.words.set(true);
    fixture.detectChanges();
    const words = el.querySelector('.bonus-chip__words')!;
    expect(words.textContent).toBe('Bônus mágico, já somado');
    expect(words.closest('[aria-hidden="true"]')).toBeNull();
    expect(el.querySelector('[aria-label]')).toBeNull();
    host.counts.set(false);
    fixture.detectChanges();
    expect(el.querySelector('.bonus-chip__words')!.textContent).toBe(
      'Bônus mágico, não conta agora',
    );
  });
});
