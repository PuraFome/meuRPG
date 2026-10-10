import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { DiceRollSchema } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { AdvantageSourceSchema, RollMode } from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import { dieFields } from '../../../../core/effects/effects';
import type { HintTry } from '../../../../core/puzzles/puzzle-play';
import type { HintDie } from '../../../../core/puzzles/puzzles-client';
import { HintTryControl } from './hint-try';

describe('HintTryControl: the d20 pair', () => {
  function setup(inputs: Record<string, unknown>) {
    const fixture = TestBed.createComponent(HintTryControl);
    fixture.componentRef.setInput('skill', 'Investigação');
    fixture.componentRef.setInput('canTry', true);
    fixture.componentRef.setInput('diceMode', DiceMode.PHYSICAL);
    fixture.componentRef.setInput('dicePreference', DicePreference.PHYSICAL);
    for (const [k, v] of Object.entries(inputs)) {
      fixture.componentRef.setInput(k, v);
    }
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const sent: HintDie[] = [];
    fixture.componentInstance.typed.subscribe((d) => sent.push(d));
    return { fixture, el, sent };
  }
  const button = (el: HTMLElement, text: string) =>
    Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
      b.textContent?.includes(text),
    )!;
  const fill = (fixture: { detectChanges: () => void }, el: HTMLElement, i: number, v: string) => {
    const field = el.querySelectorAll<HTMLInputElement>('input')[i];
    field.value = v;
    field.dispatchEvent(new Event('input'));
    fixture.detectChanges();
  };

  it('asks for one die by default and sends its face', () => {
    const { fixture, el, sent } = setup({});
    button(el, 'Tentar uma dica').click();
    fixture.detectChanges();
    expect(el.querySelectorAll('input')).toHaveLength(1);
    fill(fixture, el, 0, '9');
    button(el, 'Confirmar').click();
    expect(sent).toEqual([{ face: 9 }]);
  });

  it('asks for the d4 of an effect the server said the try takes, with the d20 in the same form', () => {
    const { fixture, el, sent } = setup({
      extraFields: dieFields([{ name: 'Outra fonte', faces: 4, sign: 1 }]),
    });
    expect(Array.from(el.querySelectorAll('label'), (l) => l.textContent?.trim())).toContain(
      'Resultado do d4 (Outra fonte)',
    );
    fill(fixture, el, 0, '3');
    fill(fixture, el, 1, '9');
    button(el, 'Confirmar').click();
    expect(sent).toEqual([{ face: 9 }]);
    expect(fixture.componentInstance.extraFaces()).toEqual([3]);
    // The form stays open for the answer: the page may ask again.
    expect(el.querySelectorAll('input')).toHaveLength(2);
  });

  it('shows two labelled fields once the pair is needed, and sends both faces', () => {
    const { fixture, el, sent } = setup({ pair: true });
    fixture.detectChanges();
    expect(Array.from(el.querySelectorAll('label'), (l) => l.textContent?.trim())).toEqual([
      'Primeiro d20',
      'Segundo d20',
    ]);
    fill(fixture, el, 0, '4');
    fill(fixture, el, 1, '18');
    el.querySelector('form')!.dispatchEvent(new Event('submit'));
    expect(sent).toEqual([{ faces: [4, 18] }]);
  });

  it('switches to two fields when the pair arrives after the first try', () => {
    const { fixture, el } = setup({});
    button(el, 'Tentar uma dica').click();
    fixture.detectChanges();
    expect(el.querySelectorAll('input')).toHaveLength(1);
    fixture.componentRef.setInput('pair', true);
    fixture.detectChanges();
    expect(el.querySelectorAll('input')).toHaveLength(2);
  });

  it('shows the mode, the pair with the counted die and the sources of the last try', () => {
    const result: HintTry = {
      passed: true,
      mode: RollMode.ADVANTAGE,
      sources: [create(AdvantageSourceSchema, { textPt: 'Fúria: vantagem em testes de Força' })],
      roll: create(DiceRollSchema, {
        diceCount: 2,
        diceSides: 20,
        faces: [6, 16],
        modifier: 3,
        total: 19,
        countedIndex: 1,
      }),
    };
    const { el } = setup({ result, canTry: false });
    expect(el.querySelector('.mode__word')?.textContent).toBe('Vantagem');
    expect(
      Array.from(el.querySelectorAll('.die'), (d) => d.textContent?.replace(/\s+/g, ' ').trim()),
    ).toEqual(['6 não conta', '16 conta']);
    expect(el.querySelector('.mode__sources')?.textContent).toContain('Fúria');
  });
});
