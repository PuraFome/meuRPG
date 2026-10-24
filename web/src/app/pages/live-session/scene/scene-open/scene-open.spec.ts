import { TestBed } from '@angular/core/testing';

import { MapState } from '../../../../core/maps/map-state';
import { mapMessage, mapPoint, mapResponse } from '../../../../core/maps/maps-testing';
import { SceneClient } from '../../../../core/play/scene-client';
import { SceneState } from '../../../../core/play/scene-state';
import { FakeSceneClient, masterScene, sceneRoll } from '../../../../core/play/scene-testing';
import { LiveSessionSource } from '../../../../core/play/live-session.types';
import { SceneOpen } from './scene-open';

const flat = (e: Element | null | undefined) =>
  e?.textContent
    ?.replace(/\u00a0/g, ' ')
    .replace(/\s+/g, ' ')
    .trim();

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
    const state = new SceneState(
      () => api.get(),
      () => true,
    );
    state.apply(scene);
    const mapState = new MapState(() =>
      Promise.resolve(
        mapResponse(mapMessage('m1', 'Estrada do Vale'), [mapPoint('p1', 'A carroça tombada')]),
      ),
    );
    await mapState.open('m1');
    TestBed.configureTestingModule({
      providers: [
        { provide: SceneClient, useValue: api },
        { provide: LiveSessionSource, useValue: { setShownImage: async () => null } },
      ],
    });
    const fixture = TestBed.createComponent(SceneOpen);
    fixture.componentRef.setInput('campaignId', 'c1');
    fixture.componentRef.setInput('state', state);
    fixture.componentRef.setInput('mapState', mapState);
    fixture.detectChanges();
    return { fixture, api, state, el: fixture.nativeElement as HTMLElement };
  }

  it("shows the scene's images as thumbnails with a place for the gallery tile, even when it has none", async () => {
    const none = await setup();
    expect(none.el.querySelector('app-scene-images-show h3')?.textContent).toContain(
      'Imagens da cena',
    );
    expect(
      none.el.querySelectorAll('app-scene-images-show .sm__tile:not(.sm__tile--add)'),
    ).toHaveLength(0);
    TestBed.resetTestingModule();
    const { el } = await setup(
      masterScene([], [], {
        images: [
          { id: 'i1', name: 'A carroça', showsWholeMap: false },
          { id: 'i2', name: 'A estrada', showsWholeMap: false },
        ],
      }),
    );
    expect(el.querySelectorAll('app-scene-images-show .sm__tile:not(.sm__tile--add)')).toHaveLength(
      2,
    );
  });

  it('draws the parts of the scene in the order of the board', async () => {
    const { el } = await setup();
    const parts = Array.from(
      el.querySelectorAll(
        '.so > app-stage-master, .so > .so__images, .so > app-scene-actions, .so > app-scene-clues, .so > app-scene-hooks',
      ),
      (c) => (c.tagName === 'DIV' ? 'images' : c.tagName.toLowerCase().replace('app-', '')),
    );
    expect(parts).toEqual([
      'stage-master',
      'images',
      'scene-actions',
      'scene-clues',
      'scene-hooks',
    ]);
  });

  it('shows the title, when it opened, where it is and how much has happened', async () => {
    const { el } = await setup();
    expect(el.querySelector('h2')?.textContent).toBe('Cena: A\u00a0carroça tombada');
    expect(flat(el.querySelector('.so__since'))).toContain('Aberta para os jogadores desde 21:10');
    expect(flat(el.querySelector('.so__meta'))).toBe('Mapa Estrada do Vale · 5 ações · 3 rolagens');
  });

  it('says there is no roll yet in the meta line', async () => {
    const { el } = await setup(masterScene());
    expect(el.querySelector('.so__meta')?.textContent).toContain('nenhuma rolagem');
  });

  it('closes the scene asking nothing, and hands focus to "Abrir cena"', async () => {
    const { fixture, api, state, el } = await setup();
    const close = Array.from(el.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('Fechar cena'),
    )!;
    close.click();
    for (let i = 0; i < 3; i++) {
      await fixture.whenStable();
    }
    expect(api.calls).toEqual(['close']);
    expect(state.scene()).toBeNull();
    expect(state.focusNext()).toBe('open');
  });

  it('has "Trocar cena" and "Fechar cena" as text actions with their icons, not outlined buttons', async () => {
    const { el } = await setup();
    const buttons = Array.from(el.querySelectorAll<HTMLElement>('.so__buttons button'));
    expect(buttons.map((b) => flat(b))).toEqual(['swap_horizTrocar cena', 'closeFechar cena']);
    expect(buttons.some((b) => b.classList.contains('mat-mdc-outlined-button'))).toBe(false);
    expect(buttons.some((b) => b.classList.contains('mat-mdc-unelevated-button'))).toBe(false);
  });

  it('reads the new roll aloud in a polite region', async () => {
    const { fixture, el, state } = await setup(masterScene([toren]));
    state.apply(masterScene([pens, toren]));
    fixture.detectChanges();
    const region = el.querySelector('.so__live');
    expect(region?.getAttribute('aria-live')).toBe('polite');
    expect(region?.textContent).toBe('Pensantus: Procurar pistas na carroça, 17, passou');
  });
});
