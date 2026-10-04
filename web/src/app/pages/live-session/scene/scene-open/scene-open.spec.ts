import { TestBed } from '@angular/core/testing';

import { MapState } from '../../../../core/maps/map-state';
import { mapMessage, mapPoint, mapResponse } from '../../../../core/maps/maps-testing';
import { SceneClient } from '../../../../core/play/scene-client';
import { SceneState } from '../../../../core/play/scene-state';
import { FakeSceneClient, masterScene, sceneRoll } from '../../../../core/play/scene-testing';
import { SceneOpen } from './scene-open';

const flat = (e: Element | null | undefined) => e?.textContent?.replace(/ /g, ' ').replace(/\s+/g, ' ').trim();

describe('SceneOpen', () => {
  const toren = sceneRoll('r1', 'a2', 'Toren', 7, { passed: false });
  const brisa = sceneRoll('r2', 'a4', 'Brisa', 17, {
    roll: { diceCount: 1, diceSides: 20, faces: [14], modifier: 3, total: 17, physical: true },
  });
  const pens = sceneRoll('r3', 'a1', 'Pensantus', 17, {
    passed: true,
    roll: { diceCount: 1, diceSides: 20, faces: [11], modifier: 6, total: 17 },
  });

  async function setup(scene = masterScene([toren, brisa, pens])) {
    const api = new FakeSceneClient();
    const state = new SceneState(() => api.get(), () => true);
    state.apply(scene);
    const mapState = new MapState(() =>
      Promise.resolve(mapResponse(mapMessage('m1', 'Estrada do Vale'), [mapPoint('p1', 'A carroça tombada')])),
    );
    await mapState.open('m1');
    TestBed.configureTestingModule({ providers: [{ provide: SceneClient, useValue: api }] });
    const fixture = TestBed.createComponent(SceneOpen);
    fixture.componentRef.setInput('campaignId', 'c1');
    fixture.componentRef.setInput('state', state);
    fixture.componentRef.setInput('mapState', mapState);
    fixture.detectChanges();
    return { fixture, api, state, el: fixture.nativeElement as HTMLElement };
  }

  const lines = (el: HTMLElement) => Array.from(el.querySelectorAll('app-scene-roll-line'));

  it('shows the title, when it opened, where it is and how much has happened', async () => {
    const { el } = await setup();
    expect(el.querySelector('h2')?.textContent).toBe('Cena: A\u00a0carroça tombada');
    expect(flat(el.querySelector('.so__since'))).toContain('Aberta para os jogadores desde 21:10');
    expect(flat(el.querySelector('.so__meta'))).toBe('Mapa Estrada do Vale · 5 ações · 3 rolagens');
  });

  it('lists the rolls newest first, each with its formula', async () => {
    const { el } = await setup();
    expect(lines(el).map((l) => flat(l.querySelector('.rl__who')))).toEqual(['Toren', 'Brisa', 'Pensantus']);
    expect(lines(el).map((l) => flat(l.querySelector('.rl__formula')))).toEqual([
      '1d20 (6) + 1 = 7', '14 + 3 = 17', '1d20 (11) + 6 = 17',
    ]);
    expect(flat(lines(el)[0].querySelector('.rl__time'))).toBe('21:14');
  });

  it('says passed or not with the DC, on its own line under the formula, and nothing without a DC', async () => {
    const { el } = await setup();
    const [failed, noDc, passed] = lines(el);
    expect(flat(failed.querySelector('.mr-tag'))).toContain('Não passou · CD 13');
    expect(failed.querySelector('.mr-tag--danger mat-icon')?.textContent).toBe('close');
    expect(flat(passed.querySelector('.mr-tag'))).toContain('Passou · CD 12');
    expect(passed.querySelector('.mr-tag--success mat-icon')?.textContent).toBe('check');
    expect(noDc.querySelector('.mr-tag')).toBeNull();
    // The pill follows the formula in the DOM, in the same column.
    const body = failed.querySelector('.rl__body')!;
    expect(Array.from(body.children).map((c) => c.className.split(' ')[0])).toEqual(['rl__top', 'rl__action', 'rl__formula', 'mr-tag']);
    // A typed die is named on the action line, not in the pill slot.
    expect(flat(noDc.querySelector('.rl__action'))).toBe('Percepção · dado físico');
  });

  it('lists the actions with their DCs, which only the master sees', async () => {
    const { el } = await setup();
    const items = Array.from(el.querySelectorAll('.so__action'));
    expect(items).toHaveLength(5);
    expect(items.map((i) => flat(i.querySelector('.mr-tag')) ?? null)).toEqual(['CD 12', 'CD 13', null, null, 'CD 10']);
    expect(Array.from(items[3].children, (c) => c.textContent)).toEqual(['Percepção', 'Perícia']);
    expect(flat(el.querySelector('.so__hint'))).toBe('Chegam aqui na hora, a mais nova em cima');
  });

  it('says there is no roll yet', async () => {
    const { el } = await setup(masterScene());
    expect(el.textContent).toContain('Ninguém rolou ainda.');
    expect(el.querySelector('.so__meta')?.textContent).toContain('nenhuma rolagem');
  });

  it('closes the scene asking nothing, and hands focus to "Abrir cena"', async () => {
    const { fixture, api, state, el } = await setup();
    const close = Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.includes('Fechar cena'))!;
    close.click();
    for (let i = 0; i < 3; i++) {
      await fixture.whenStable();
    }
    expect(api.calls).toEqual(['close']);
    expect(state.scene()).toBeNull();
    expect(state.focusNext()).toBe('open');
  });

  it('keeps both buttons outlined (the one filled button is not here)', async () => {
    const { el } = await setup();
    const buttons = Array.from(el.querySelectorAll<HTMLElement>('.so__buttons button'));
    expect(buttons.map((b) => flat(b))).toEqual(['swap_horizTrocar cena', 'closeFechar cena']);
    expect(buttons.every((b) => b.classList.contains('mat-mdc-outlined-button'))).toBe(true);
  });

  it('reads the new roll aloud in a polite region', async () => {
    const { fixture, el, state } = await setup(masterScene([toren]));
    state.apply(masterScene([pens, toren]));
    fixture.detectChanges();
    const region = el.querySelector('.so__live');
    expect(region?.getAttribute('aria-live')).toBe('polite');
    expect(region?.textContent).toBe('Pensantus: Procurar pistas na carroça, 17, passou');
    expect(lines(el)).toHaveLength(2);
  });
});
