import { TestBed } from '@angular/core/testing';

import { DamageParts } from './damage-parts';

const parts = [
  {
    key: 'weapon',
    labelPt: 'Espada curta',
    diceCount: 2,
    diceSides: 6,
    flat: 3,
    damageTypePt: 'perfurante',
    doubled: true,
  },
  {
    key: 'rage',
    labelPt: 'Fúria',
    diceCount: 0,
    diceSides: 0,
    flat: 2,
    auto: true,
    damageTypePt: 'perfurante',
  },
  {
    key: 'sneak-attack',
    labelPt: 'Ataque Furtivo',
    diceCount: 4,
    diceSides: 6,
    flat: 0,
    choosable: true,
    selected: true,
    available: true,
    doubled: true,
    reasonPt: 'Você tem vantagem no ataque.',
  },
  {
    key: 'hunters-mark',
    labelPt: 'Marca do Caçador',
    diceCount: 1,
    diceSides: 6,
    flat: 0,
    choosable: true,
    available: false,
    reasonPt: 'O alvo não está marcado.',
  },
  {
    key: 'divine-smite',
    labelPt: 'Golpe Divino',
    diceCount: 2,
    diceSides: 8,
    flat: 0,
    choosable: true,
    available: true,
    needsSlot: true,
    slotOptions: [
      { level: 1, pact: false, free: 0, max: 2, diceCount: 2 },
      { level: 2, pact: false, free: 1, max: 2, diceCount: 3 },
    ],
  },
] as never[];

function setup() {
  const fixture = TestBed.createComponent(DamageParts);
  fixture.componentRef.setInput('parts', parts);
  fixture.componentRef.setInput('picks', [{ key: 'sneak-attack', slotLevel: 0, pact: false }]);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const box = (label: string) =>
    Array.from(el.querySelectorAll<HTMLElement>('.extra')).find((e) =>
      e.textContent!.includes(label),
    )!;
  return { fixture, el, box };
}

describe('DamageParts', () => {
  it('lists the weapon and the automatic lines as fixed lines, doubling only the dice', () => {
    const { el } = setup();
    const lines = Array.from(el.querySelectorAll('.fixed .line')).map((l) =>
      l.textContent!.replace(/\s+/g, ' ').trim(),
    );
    expect(lines[0]).toBe('Espada curta 2d6 + 3 de perfurante dados dobrados');
    expect(lines[1]).toBe('Fúria 2 de perfurante automático');
    expect(lines[1]).not.toContain('dobrados');
  });

  it('lists the extras as checkboxes, the marked one checked', () => {
    const { box } = setup();
    expect(box('Ataque Furtivo').querySelector<HTMLInputElement>('input')!.checked).toBe(true);
    expect(box('Ataque Furtivo').textContent).toContain('Você tem vantagem no ataque.');
  });

  it('disables an extra that does not hold and says why in words', () => {
    const { box } = setup();
    const unavailable = box('Marca do Caçador');
    expect(unavailable.querySelector<HTMLInputElement>('input')!.disabled).toBe(true);
    expect(unavailable.textContent!.replace(/\s+/g, ' ')).toContain(
      'Indisponível: O alvo não está marcado.',
    );
  });

  it('marks and unmarks an extra', () => {
    const { fixture, box } = setup();
    box('Ataque Furtivo').querySelector('input')!.click();
    fixture.detectChanges();
    expect(fixture.componentInstance.picks()).toEqual([]);
  });

  it('asks which slot Golpe Divino spends, the lowest free one first, a used-up slot disabled', () => {
    const { fixture, box } = setup();
    expect(box('Golpe Divino').querySelector('[role="radiogroup"]')).toBeNull();
    box('Golpe Divino').querySelector('input')!.click();
    fixture.detectChanges();
    expect(fixture.componentInstance.picks().find((p) => p.key === 'divine-smite')).toEqual({
      key: 'divine-smite',
      slotLevel: 2,
      pact: false,
    });
    const slots = Array.from(
      box('Golpe Divino').querySelectorAll<HTMLInputElement>('[role="radiogroup"] input'),
    );
    expect(slots.map((s) => s.disabled)).toEqual([true, false]);
    expect(slots[1].checked).toBe(true);
    expect(box('Golpe Divino').textContent!.replace(/\s+/g, ' ')).toContain(
      '2º nível · 1 de 2 livres · 3d8',
    );
  });
});
