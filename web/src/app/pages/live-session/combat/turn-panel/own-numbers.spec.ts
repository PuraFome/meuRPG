import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import { CombatantSchema } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { OwnNumbers } from './own-numbers';

describe('OwnNumbers', () => {
  function render(inputs: Record<string, unknown>, own = {}) {
    const fixture = TestBed.createComponent(OwnNumbers);
    fixture.componentRef.setInput(
      'own',
      create(CombatantSchema, { hitPointsCurrent: 19, hitPointsMax: 28, ...own }),
    );
    for (const [k, v] of Object.entries(inputs)) {
      fixture.componentRef.setInput(k, v);
    }
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    return { el, text: (el.textContent ?? '').replace(/\s+/g, ' ').trim() };
  }

  it('says the hit points and the armor class', () => {
    const { text } = render({ armorClass: 17 });
    expect(text).toContain('PV 19 de 28');
    expect(text).toContain('CA 17');
  });

  it('leaves out what it does not know, and the slots of one who casts nothing', () => {
    const { el, text } = render({}, { hitPointsCurrent: undefined, hitPointsMax: undefined });
    expect(text).not.toContain('PV');
    expect(text).not.toContain('CA');
    expect(el.querySelector('.slot')).toBeNull();
  });

  it("reads the player's hit points from its vitals when the combatant has none", () => {
    const { text } = render(
      { hitPoints: { current: 23, max: 23 } },
      { hitPointsCurrent: undefined, hitPointsMax: undefined },
    );
    expect(text).toContain('PV 23 de 23');
  });

  it('draws each circle of slots with its words', () => {
    const { el } = render({
      slots: {
        usage: [
          { level: 1, total: 4, used: 1 },
          { level: 2, total: 2, used: 0 },
          { level: 3, total: 0, used: 0 },
        ],
        pact: null,
      },
    });
    const slots = [...el.querySelectorAll('.slot')];
    expect(slots.map((s) => s.getAttribute('aria-label'))).toEqual([
      '1º nível: 3 livres de 4',
      '2º nível: 2 livres de 2',
    ]);
    expect(slots[0].querySelectorAll('svg')).toHaveLength(4);
    // The free ones in numbers beside the dots, not left to be counted.
    expect(slots.map((s) => s.querySelector('.slot__free')?.textContent)).toEqual([
      '3 de 4',
      '2 de 2',
    ]);
  });
});
