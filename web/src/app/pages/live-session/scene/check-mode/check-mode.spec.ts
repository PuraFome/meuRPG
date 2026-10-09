import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import { AdvantageSourceSchema, RollMode } from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import { CheckMode } from './check-mode';

function setup(inputs: Record<string, unknown>) {
  const fixture = TestBed.createComponent(CheckMode);
  for (const [k, v] of Object.entries(inputs)) {
    fixture.componentRef.setInput(k, v);
  }
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('CheckMode', () => {
  it('names the mode, marks the counted die in words and lists the sources', () => {
    const el = setup({
      mode: RollMode.DISADVANTAGE,
      faces: [
        { value: 3, counts: true },
        { value: 12, counts: false },
      ],
      sources: [create(AdvantageSourceSchema, { textPt: 'Envenenado: desvantagem em testes' })],
    });
    expect(el.querySelector('.mode__word')?.textContent).toBe('Desvantagem');
    const dice = Array.from(el.querySelectorAll('.die')).map((d) =>
      d.textContent?.replace(/\s+/g, ' ').trim(),
    );
    expect(dice).toEqual(['3 conta', '12 não conta']);
    expect(el.querySelector('.die--counts')?.textContent).toContain('3');
    expect(el.querySelector('.mode__sources')?.textContent).toContain(
      'Envenenado: desvantagem em testes',
    );
  });

  it('draws nothing for a normal single die with no source', () => {
    const el = setup({ mode: RollMode.NORMAL });
    expect(el.textContent?.trim()).toBe('');
  });
});
