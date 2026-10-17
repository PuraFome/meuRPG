import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { DiceRollSchema } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { AdvantageSourceSchema, RollMode } from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import { dieFields } from '../../../../core/effects/effects';
import type { HintTry } from '../../../../core/puzzles/puzzle-play';
import type { HintDie } from '../../../../core/puzzles/puzzles-client';
import { LUCKY_TRAIT } from '../../../../core/combat/lucky';
import { heldDie, heldLucky } from '../../../../core/held-rolls/held-roll-testing';
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

describe('HintTryControl: the Bardic Inspiration question', () => {
  it('shows the question in place of the roll form and emits the answer, never a DC', () => {
    const fixture = TestBed.createComponent(HintTryControl);
    fixture.componentRef.setInput('skill', 'Investigação');
    fixture.componentRef.setInput('canTry', true);
    fixture.componentRef.setInput('held', heldDie(9, 3));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const answers: unknown[] = [];
    fixture.componentInstance.answered.subscribe((a) => answers.push(a));
    expect(el.textContent).toContain('Usar a Inspiração de Bardo (d8)?');
    expect(el.textContent).not.toContain('Tentar uma dica');
    expect(el.textContent).not.toMatch(/\bCD\b/);
    Array.from(el.querySelectorAll('button'))
      .find((b) => b.textContent?.includes('Usar e rolar o d8'))!
      .click();
    expect(answers).toEqual([{ kind: 'die', use: true, roll: { inApp: true } }]);
  });
});

describe('HintTryControl: the halfling’s Lucky question', () => {
  function setup(inputs: Record<string, unknown>) {
    const fixture = TestBed.createComponent(HintTryControl);
    fixture.componentRef.setInput('skill', 'Investigação');
    fixture.componentRef.setInput('canTry', true);
    for (const [k, v] of Object.entries(inputs)) {
      fixture.componentRef.setInput(k, v);
    }
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const answers: unknown[] = [];
    fixture.componentInstance.answered.subscribe((a) => answers.push(a));
    const button = (text: string) =>
      Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
        b.textContent?.includes(text),
      );
    return { el, answers, button };
  }

  it('asks it first, in place of the roll form and of the die’s question, and never says the DC', () => {
    const { el, answers, button } = setup({ held: heldLucky([1], 3) });
    expect(el.textContent).toContain('Tirou 1: rolar de novo?');
    expect(el.textContent).not.toContain('Usar a Inspiração de Bardo');
    expect(el.textContent).not.toContain('Tentar uma dica');
    expect(el.textContent).not.toMatch(/\bCD\b/);
    button('Rolar de novo')!.click();
    button('Ficar com o 1')!.click();
    expect(answers).toEqual([
      { kind: 'lucky', reroll: true, roll: { inApp: true } },
      { kind: 'lucky', reroll: false, roll: null },
    ]);
  });

  it('says "Sortudo: 1 → 14" with the result of a try whose d20 Lucky rolled again', () => {
    const result: HintTry = {
      passed: true,
      roll: create(DiceRollSchema, {
        diceCount: 1,
        diceSides: 20,
        faces: [14],
        modifier: 3,
        total: 17,
        rerolled: [{ index: 0, from: 1, to: 14, sourceKey: LUCKY_TRAIT }],
      }),
      mode: RollMode.NORMAL,
      sources: [],
    };
    const { el } = setup({ result, canTry: false });
    expect(el.querySelector('[data-testid="lucky"]')?.textContent?.trim()).toBe('Sortudo: 1 → 14.');
  });
});
