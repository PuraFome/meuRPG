import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import {
  LevelUpNamedKeySchema,
  LevelUpOptionsSchema,
  LevelUpSwapGroupSchema,
} from '../../../../gen/meurpg/characters/v1/characters_pb';
import { LevelUpSwap } from '../../../core/levelup/levelup-swap';
import { SwapPicker } from './swap-picker';

const named = (key: string, namePt: string) => create(LevelUpNamedKeySchema, { key, namePt });

function swapOf(withGroups = true): LevelUpSwap {
  return new LevelUpSwap(
    create(LevelUpOptionsSchema, {
      swapGroups: withGroups
        ? [
            create(LevelUpSwapGroupSchema, {
              group: 'fighting_style',
              namePt: 'Estilo de luta',
              picks: [named('feature:fighter-fighting-style-defense', 'Defesa')],
              pool: [
                named('feature:fighter-fighting-style-dueling', 'Duelismo'),
                named('option:fighting_style:guarda-dupla@mesa', 'Guarda Dupla'),
              ],
            }),
          ]
        : [],
    }),
  );
}

function render(swap: LevelUpSwap) {
  const fixture = TestBed.createComponent(SwapPicker);
  fixture.componentRef.setInput('swap', swap);
  fixture.detectChanges();
  return fixture;
}

const radio = (el: HTMLElement, name: string, label: string) =>
  Array.from(el.querySelectorAll<HTMLInputElement>(`input[name="${name}"]`)).find((i) =>
    i.closest('label')?.textContent?.includes(label),
  )!;

describe('LevelUpSwap and SwapPicker', () => {
  it('has nothing to send and no groups when the table does not use the rule', () => {
    const swap = swapOf(false);
    expect(swap.groups()).toEqual([]);
    expect(swap.pick()).toBeUndefined();
  });

  it('hides the picks until a group is chosen', () => {
    const el = render(swapOf()).nativeElement as HTMLElement;
    expect(el.querySelector('input[name="swap-from"]')).toBeNull();
    expect(radio(el, 'swap-group', 'Não trocar').checked).toBe(true);
  });

  it('puts the swap in the choices and lists it', () => {
    const swap = swapOf();
    const fixture = render(swap);
    const el = fixture.nativeElement as HTMLElement;
    radio(el, 'swap-group', 'Estilo de luta').click();
    fixture.detectChanges();
    radio(el, 'swap-from', 'Defesa').click();
    fixture.detectChanges();
    expect(swap.pick()).toBeUndefined();
    radio(el, 'swap-to', 'Guarda Dupla').click();
    fixture.detectChanges();
    expect(swap.pick()).toEqual({
      group: 'fighting_style',
      fromKey: 'feature:fighter-fighting-style-defense',
      toKey: 'option:fighting_style:guarda-dupla@mesa',
    });
    expect(swap.summary()).toBe('Trocar Defesa por Guarda Dupla');
  });

  it('"Não trocar" clears the swap', () => {
    const swap = swapOf();
    const fixture = render(swap);
    const el = fixture.nativeElement as HTMLElement;
    swap.setGroup('fighting_style');
    swap.setFrom('feature:fighter-fighting-style-defense');
    swap.setTo('feature:fighter-fighting-style-dueling');
    fixture.detectChanges();
    expect(swap.pick()).toBeDefined();
    radio(el, 'swap-group', 'Não trocar').click();
    expect(swap.pick()).toBeUndefined();
    expect(swap.started()).toBe(false);
  });

  it('refuses a pick the group does not offer and drops the picks when the group changes', () => {
    const swap = swapOf();
    swap.setGroup('fighting_style');
    swap.setFrom('feature:other');
    expect(swap.fromKey()).toBe('');
    swap.setFrom('feature:fighter-fighting-style-defense');
    swap.setGroup('cantrip');
    expect(swap.groupKey()).toBe('');
    expect(swap.fromKey()).toBe('');
  });
});
