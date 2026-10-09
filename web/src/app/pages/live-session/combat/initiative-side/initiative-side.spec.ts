import { TestBed } from '@angular/core/testing';

import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { InitiativeSide } from './initiative-side';

const flat = (e: Element | null | undefined) => e?.textContent?.replace(/\s+/g, ' ').trim();

describe('InitiativeSide', () => {
  function render(initiatives: Record<string, number | undefined>): HTMLElement {
    const fixture = TestBed.createComponent(InitiativeSide);
    fixture.componentRef.setInput(
      'encounter',
      encounter({
        combatants: Object.entries(initiatives).map(([label, initiative]) =>
          combatant({ id: label.toLowerCase(), label, initiative }),
        ),
      }),
    );
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('says who has not rolled once in the notice and once as the reason under the button', () => {
    const el = render({ Toren: undefined, Brisa: undefined, Goblin: 12 });
    expect(flat(el.querySelector('.mr-notice--warning'))).toContain(
      'Toren e Brisa ainda não rolaram.',
    );
    expect(flat(el.querySelector('.side__reason'))).toBe('Faltam as iniciativas de Toren e Brisa.');
    // The facts list is not a third copy of the same sentence.
    expect(flat(el.querySelector('.side__facts'))).not.toContain('Faltam as iniciativas');
    expect(flat(el.querySelector('.side__facts'))).toContain('1 de 3 rolaram');
  });

  it('has no reason once everyone rolled', () => {
    const el = render({ Toren: 14, Goblin: 12 });
    expect(el.querySelector('.side__reason')).toBeNull();
    expect(flat(el.querySelector('.mr-notice--success'))).toContain('Todos rolaram.');
  });
});
