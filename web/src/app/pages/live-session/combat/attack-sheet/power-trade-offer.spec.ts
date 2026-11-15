import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import { PowerAttackOptionSchema } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { PowerTradeOffer } from './power-trade-offer';

const plain = (t: string | null | undefined) => (t ?? '').replace(/\s+/g, ' ').trim();
const heavy = create(PowerAttackOptionSchema, {
  key: 'feat:golpe-pesado@mesa',
  name: 'Golpe Pesado',
  penalty: 5,
  damageBonus: 10,
});
const steady = create(PowerAttackOptionSchema, {
  key: 'feat:disparo-firme@mesa',
  name: 'Disparo Firme',
  penalty: 5,
  damageBonus: 10,
  disabled: true,
  disabledReasonPt: 'Sem usos de Fôlego do Atirador: voltam no descanso curto.',
});
const box = {
  attackBefore: '1d20 + 6',
  attackAfter: '1d20 + 1',
  damageBefore: '2d6 + 3',
  damageAfter: '2d6 + 13 cortante, se acertar',
};

describe('PowerTradeOffer', () => {
  function setup(inputs: { trades?: unknown[]; pickedKey?: string; box?: typeof box | null } = {}) {
    const fixture = TestBed.createComponent(PowerTradeOffer);
    fixture.componentRef.setInput('trades', inputs.trades ?? [heavy]);
    fixture.componentRef.setInput('pickedKey', inputs.pickedKey ?? '');
    fixture.componentRef.setInput('box', inputs.box ?? null);
    const chosen: string[] = [];
    fixture.componentInstance.chosen.subscribe((k) => chosen.push(k));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    return {
      fixture,
      el,
      chosen,
      boxes: () => [...el.querySelectorAll<HTMLInputElement>('input')],
    };
  }

  it('names the group "Troca do talento" and offers "Usar Golpe Pesado", off, with what it costs and gives', () => {
    const { el, boxes } = setup();
    expect(boxes().length).toBe(1);
    expect(plain(el.querySelector('legend')?.textContent)).toBe('Troca do talento');
    expect(plain(el.textContent)).toContain('Usar Golpe Pesado');
    expect(plain(el.textContent)).toContain('−5 no ataque, +10 no dano se acertar');
    expect(boxes()[0].type).toBe('checkbox');
    expect(boxes()[0].checked).toBe(false);
    expect(el.querySelector('.box')).toBeNull();
  });

  it('says the key to the page when the check is pressed', () => {
    const { boxes, chosen } = setup();
    expect(boxes().length).toBe(1);
    boxes()[0].click();
    expect(chosen).toEqual(['feat:golpe-pesado@mesa']);
  });

  it('ticks the trade picked and shows the box with what it was and what it is, the old numbers struck', () => {
    const { el, boxes } = setup({ pickedKey: heavy.key, box });
    expect(boxes().length).toBe(1);
    expect(boxes()[0].checked).toBe(true);
    const text = plain(el.querySelector('.box')?.textContent);
    expect(text).toContain('Ataque');
    expect(text).toContain('1d20 + 6');
    expect(text).toContain('1d20 + 1');
    expect(text).toContain('Dano');
    expect(text).toContain('2d6 + 3');
    expect(text).toContain('2d6 + 13 cortante, se acertar');
    expect([...el.querySelectorAll('.box s')].map((x) => plain(x.textContent))).toEqual([
      '1d20 + 6',
      '2d6 + 3',
    ]);
  });

  it('keeps the box a live region, so the new numbers are read when the check is pressed', () => {
    const { el } = setup({ pickedKey: heavy.key, box });
    expect(el.querySelector('[aria-live="polite"]')).not.toBeNull();
  });

  it('shows a trade with no use left off, with the reason the server sent, and never says it is chosen', () => {
    const { el, boxes, chosen } = setup({ trades: [steady] });
    expect(boxes().length).toBe(1);
    expect(boxes()[0].disabled).toBe(true);
    expect(plain(el.textContent)).toContain(
      'Sem usos de Fôlego do Atirador: voltam no descanso curto.',
    );
    boxes()[0].click();
    expect(chosen).toEqual([]);
  });
});
