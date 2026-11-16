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

const FEATURE = 'feature:infusoes@mesa';

/** The table rule's group and one table feature's group, as a level with the ability step offers both. */
function bothOf(): LevelUpSwap {
  const group = (key: string, namePt: string, from: string, to: string[]) =>
    create(LevelUpSwapGroupSchema, {
      group: key,
      namePt,
      picks: [named(`${key}:${from}`, from)],
      pool: to.map((n) => named(`${key}:${n}`, n)),
    });
  return new LevelUpSwap(
    create(LevelUpOptionsSchema, {
      swapGroups: [
        group('fighting_style', 'Estilo de luta', 'Defesa', ['Duelismo']),
        group('cantrip', 'Truque', 'Luz', ['Chama']),
        group(FEATURE, 'Infusões', 'Escudo', ['Radiante', 'Bumerangue']),
        group('feature:outra@mesa', 'Outra', 'X', ['Y']),
      ],
    }),
  );
}

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

  it('holds one selection for each group and keeps them when another group is opened', () => {
    const swap = bothOf();
    swap.setGroup(FEATURE);
    swap.setFrom(`${FEATURE}:Escudo`);
    swap.setTo(`${FEATURE}:Radiante`);
    swap.setGroup('feature:outra@mesa');
    swap.setFrom('feature:outra@mesa:X');
    swap.setTo('feature:outra@mesa:Y');
    swap.setGroup(FEATURE);
    expect(swap.fromKey()).toBe(`${FEATURE}:Escudo`);
    expect(swap.featurePicks()).toEqual([
      { group: FEATURE, fromKey: `${FEATURE}:Escudo`, toKey: `${FEATURE}:Radiante` },
      {
        group: 'feature:outra@mesa',
        fromKey: 'feature:outra@mesa:X',
        toKey: 'feature:outra@mesa:Y',
      },
    ]);
    expect(swap.pick()).toBeUndefined();
    expect(swap.summaries()).toEqual(['Trocar Escudo por Radiante', 'Trocar X por Y']);
  });

  it('tells a table feature group from a rule group by the feature: key, and sends them apart', () => {
    const swap = bothOf();
    swap.setGroup('fighting_style');
    swap.setFrom('fighting_style:Defesa');
    swap.setTo('fighting_style:Duelismo');
    swap.setGroup(FEATURE);
    swap.setFrom(`${FEATURE}:Escudo`);
    swap.setTo(`${FEATURE}:Bumerangue`);
    expect(swap.pick()).toEqual({
      group: 'fighting_style',
      fromKey: 'fighting_style:Defesa',
      toKey: 'fighting_style:Duelismo',
    });
    expect(swap.featurePicks()).toEqual([
      { group: FEATURE, fromKey: `${FEATURE}:Escudo`, toKey: `${FEATURE}:Bumerangue` },
    ]);
  });

  it('lets the rule swap one group only: starting another rule group drops the first', () => {
    const swap = bothOf();
    swap.setGroup('fighting_style');
    swap.setFrom('fighting_style:Defesa');
    swap.setTo('fighting_style:Duelismo');
    swap.setGroup('cantrip');
    swap.setFrom('cantrip:Luz');
    swap.setTo('cantrip:Chama');
    expect(swap.pick()?.group).toBe('cantrip');
    swap.setGroup('fighting_style');
    expect(swap.fromKey()).toBe('');
  });

  it('sends nothing for a feature group whose swap is half chosen, and "Não trocar" clears all', () => {
    const swap = bothOf();
    swap.setGroup(FEATURE);
    swap.setFrom(`${FEATURE}:Escudo`);
    expect(swap.featurePicks()).toEqual([]);
    swap.setTo(`${FEATURE}:Radiante`);
    expect(swap.featurePicks()).toHaveLength(1);
    swap.clear();
    expect(swap.featurePicks()).toEqual([]);
    expect(swap.started()).toBe(false);
  });

  it('takes over every selection of another draft that the new options still offer', () => {
    const old = bothOf();
    old.setGroup(FEATURE);
    old.setFrom(`${FEATURE}:Escudo`);
    old.setTo(`${FEATURE}:Radiante`);
    old.setGroup('fighting_style');
    old.setFrom('fighting_style:Defesa');
    const fresh = bothOf();
    fresh.adopt(old);
    expect(fresh.featurePicks()).toHaveLength(1);
    expect(fresh.groupKey()).toBe('fighting_style');
    expect(fresh.fromKey()).toBe('fighting_style:Defesa');
  });

  it('shows a table feature group beside the rule groups in the one panel, and marks it once swapped', () => {
    const swap = bothOf();
    const fixture = render(swap);
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelectorAll('h2')).toHaveLength(1);
    expect(radio(el, 'swap-group', 'Infusões')).toBeDefined();
    radio(el, 'swap-group', 'Infusões').click();
    fixture.detectChanges();
    radio(el, 'swap-from', 'Escudo').click();
    radio(el, 'swap-to', 'Bumerangue').click();
    fixture.detectChanges();
    expect(el.querySelector('[role="status"]')?.textContent).toContain(
      'Trocar Escudo por Bumerangue',
    );
    radio(el, 'swap-group', 'Estilo de luta').click();
    fixture.detectChanges();
    expect(
      Array.from(el.querySelectorAll('label')).find((l) => l.textContent?.includes('Infusões'))
        ?.textContent,
    ).toContain('troca escolhida');
    expect(swap.featurePicks()).toHaveLength(1);
  });
});
