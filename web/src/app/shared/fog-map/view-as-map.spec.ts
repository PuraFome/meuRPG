import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { CharacterKind } from '../../../gen/meurpg/characters/v1/characters_pb';
import { MapsClient } from '../../core/maps/maps-client';
import { FakeMapsClient, mapMessage, mapResponse, mapToken } from '../../core/maps/maps-testing';
import { visionResponse } from '../../core/maps/vision-testing';
import { ViewAsMapView } from './view-as-map';

describe('ViewAsMapView: where "Voltar" goes', () => {
  async function render(backTo?: string) {
    const api = new FakeMapsClient();
    api.visions.set('c-toren', visionResponse(['BBBB', '....']));
    api.responses.set(
      'map-1@c-toren',
      mapResponse(
        mapMessage('map-1', 'A caverna', { gridColumns: 4, gridRows: 2, fogEnabled: true }),
        [],
        [mapToken('c-toren', 'Toren')],
      ),
    );
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({ providers: [{ provide: MapsClient, useValue: api }] });
    const fixture = TestBed.createComponent(ViewAsMapView);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('mapId', 'map-1');
    fixture.componentRef.setInput('characterId', 'c-toren');
    fixture.componentRef.setInput('name', 'Toren');
    fixture.componentRef.setInput('imageWidth', 400);
    fixture.componentRef.setInput('imageHeight', 200);
    if (backTo) {
      fixture.componentRef.setInput('backTo', backTo);
    }
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return fixture;
  }
  const text = (el: HTMLElement) => (el.textContent ?? '').replace(/\s+/g, ' ');

  it('says "ao seu mapa" on the session, as it always did', async () => {
    const el = (await render()).nativeElement as HTMLElement;
    expect(text(el)).toContain('Para voltar ao seu mapa, escolha “Todos”.');
    expect(Array.from(el.querySelectorAll('button'), (b) => b.textContent?.trim())).toContain(
      'Voltar ao seu mapa',
    );
  });

  it('says "à sua vista" in the map editor', async () => {
    const fixture = await render('à sua vista');
    const el = fixture.nativeElement as HTMLElement;
    const backs: number[] = [];
    fixture.componentInstance.back.subscribe(() => backs.push(1));
    expect(text(el)).toContain('Você está vendo o mapa como Toren');
    expect(text(el)).toContain('Para voltar à sua vista, escolha “Todos”.');
    const button = Array.from(el.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Voltar à sua vista',
    )!;
    button.click();
    expect(backs).toEqual([1]);
  });
});

describe('ViewAsMapView: what the master reads while looking as a character', () => {
  const goblin = mapToken('g', 'Goblin 2', { kind: CharacterKind.MINION });
  const nanquim = mapToken('n', 'Nanquim', { kind: CharacterKind.MINION, creatureId: 'cr1' });

  async function render(tokens = [mapToken('c-toren', 'Toren'), goblin, nanquim]) {
    const api = new FakeMapsClient();
    api.visions.set('c-toren', visionResponse(['BBBB', '....']));
    api.responses.set(
      'map-1@c-toren',
      mapResponse(
        mapMessage('map-1', 'A caverna', { gridColumns: 4, gridRows: 2, fogEnabled: true }),
        [],
        tokens,
      ),
    );
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({ providers: [{ provide: MapsClient, useValue: api }] });
    const fixture = TestBed.createComponent(ViewAsMapView);
    const notes: string[] = [];
    fixture.componentInstance.noteChange.subscribe((n) => notes.push(n));
    for (const [k, v] of Object.entries({
      campaignId: 'camp-1',
      mapId: 'map-1',
      characterId: 'c-toren',
      name: 'Toren',
      imageWidth: 400,
      imageHeight: 200,
    })) {
      fixture.componentRef.setInput(k, v);
    }
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    return { fixture, api, notes };
  }

  it('says how many squares the character sees and which enemies, never counting a creature as one', async () => {
    const { notes } = await render();
    expect(notes.at(-1)).toBe('Toren vê 4 quadrados de 8 e este inimigo: Goblin 2.');
  });

  it('says "nenhum inimigo" when only a creature of the party is on the map', async () => {
    const { notes } = await render([mapToken('c-toren', 'Toren'), nanquim]);
    expect(notes.at(-1)).toBe('Toren vê 4 quadrados de 8 e nenhum inimigo.');
  });

  it('reads the vision again when the page says something may have changed, and not on the first tick', async () => {
    const { fixture, api } = await render();
    const reads = () => api.calls.filter((c) => c.startsWith('vision')).length;
    const before = reads();
    fixture.componentRef.setInput('tick', 0);
    fixture.detectChanges();
    await fixture.whenStable();
    expect(reads()).toBe(before);
    fixture.componentRef.setInput('tick', 1);
    fixture.detectChanges();
    await fixture.whenStable();
    expect(reads()).toBe(before + 1);
  });

  it("labels the view with the player's name when there is one", async () => {
    const { fixture } = await render();
    expect(fixture.componentInstance['badge']()).toBe('Vendo como Toren');
    fixture.componentRef.setInput('playerName', 'Caio');
    expect(fixture.componentInstance['badge']()).toBe('Vendo como Toren (Caio)');
  });
});
