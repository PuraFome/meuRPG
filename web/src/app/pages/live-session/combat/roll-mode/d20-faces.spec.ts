import { TestBed } from '@angular/core/testing';

import { RollMode } from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import { D20Faces } from './d20-faces';

function render(inputs: Record<string, unknown>) {
  const fixture = TestBed.createComponent(D20Faces);
  for (const [k, v] of Object.entries(inputs)) {
    fixture.componentRef.setInput(k, v);
  }
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('D20Faces', () => {
  const pair = { diceCount: 2, faces: [14, 7], countedIndex: 0 };

  it('shows both d20: the counted one marked "vale" and the other "descartado", in words', () => {
    const el = render({ roll: pair, mode: RollMode.ADVANTAGE });
    expect(el.querySelector('.d20__mode')!.textContent).toBe('Vantagem');
    const faces = Array.from(el.querySelectorAll('.face'));
    expect(faces.map((f) => f.querySelector('.face__n')!.textContent)).toEqual(['14', '7']);
    expect(faces.map((f) => f.querySelector('.face__cap')!.textContent)).toEqual([
      'vale',
      'descartado',
    ]);
    expect(faces[0].classList).toContain('face--counted');
    expect(faces[1].classList).toContain('face--dropped');
    expect(el.querySelector('.d20__faces')!.getAttribute('aria-label')).toBe(
      '14 vale, 7 descartado',
    );
  });

  it('counts the lower one for disadvantage', () => {
    const el = render({
      roll: { diceCount: 2, faces: [14, 7], countedIndex: 1 },
      mode: RollMode.DISADVANTAGE,
      caption: 'Resistência',
    });
    expect(el.querySelector('.d20__mode')!.textContent).toBe('Resistência: Desvantagem');
    expect(el.querySelector('.face--counted')!.textContent).toContain('7');
  });

  it('draws nothing for a single d20, only the reason when there is one', () => {
    const el = render({
      roll: { diceCount: 1, faces: [9], countedIndex: 0 },
      mode: RollMode.NORMAL,
    });
    expect(el.querySelector('.face')).toBeNull();
    expect(el.textContent!.trim()).toBe('');
  });

  it('says the reason, what the app suggested and the circumstances', () => {
    const el = render({
      roll: pair,
      mode: RollMode.NORMAL,
      suggested: RollMode.ADVANTAGE,
      reason: 'ele se escondeu',
      sources: [{ effect: RollMode.ADVANTAGE, textPt: 'Alvo Derrubado a 1,5 m: vantagem' }],
    });
    const text = el.textContent!.replace(/\s+/g, ' ');
    expect(text).toContain('O app sugeria Vantagem.');
    expect(text).toContain('Motivo: “ele se escondeu”');
    expect(text).toContain('Vantagem: Alvo Derrubado a 1,5 m: vantagem');
  });
});
