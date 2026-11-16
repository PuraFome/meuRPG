import { TestBed } from '@angular/core/testing';

import type { ConsumableNotice } from '../../core/characters/consumables';
import { ConsumableNoticeCard } from './consumable-notice';

describe('ConsumableNoticeCard', () => {
  function render(notice: ConsumableNotice) {
    const fixture = TestBed.createComponent(ConsumableNoticeCard);
    fixture.componentRef.setInput('notice', notice);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('says the heal in a success card, with the headline and the dice under it', () => {
    const el = render({
      tone: 'good',
      icon: 'favorite',
      headline: 'Brisa recebeu 7 PV e acorda.',
      detail: '2d4 + 2: 3 + 2 + 2 = 7. Resta 1 Poção de cura.',
      text: [],
    });
    const card = el.querySelector('.consumable__result');
    expect(card?.classList).toContain('consumable__result--good');
    expect(card?.querySelector('strong')?.textContent).toBe('Brisa recebeu 7 PV e acorda.');
    expect(card?.querySelector('.consumable__detail')?.textContent).toBe(
      '2d4 + 2: 3 + 2 + 2 = 7. Resta 1 Poção de cura.',
    );
    expect(card?.querySelector('mat-icon')?.getAttribute('aria-hidden')).toBe('true');
    expect(el.querySelector('.consumable__text')).toBeNull();
  });

  it('is neutral for an item with no automatic effect, and shows its text under the card', () => {
    const el = render({
      tone: 'neutral',
      icon: 'menu_book',
      headline: 'Tor bebeu a Poção de escalada.',
      detail: '',
      text: ['Por 1 hora, você escala.'],
    });
    // The card with only the border is the neutral look of the board: there is no class for it.
    expect(el.querySelector('.consumable__result')?.className).toBe('consumable__result');
    expect(el.querySelector('.consumable__detail')).toBeNull();
    expect(el.querySelector('.consumable__text')?.textContent).toBe('Por 1 hora, você escala.');
  });
});
