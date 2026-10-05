import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { MapsClient } from '../../core/maps/maps-client';
import { FakeMapsClient, mapMessage, mapResponse, mapToken } from '../../core/maps/maps-testing';
import { visionResponse } from '../../core/maps/vision-testing';
import { ViewAsMapView } from './view-as-map';

describe('ViewAsMapView: where "Voltar" goes', () => {
  async function render(backTo?: string) {
    const api = new FakeMapsClient();
    api.visions.set('c-toren', visionResponse(['BBBB', '....']));
    api.responses.set('map-1@c-toren', mapResponse(mapMessage('map-1', 'A caverna', { gridColumns: 4, gridRows: 2, fogEnabled: true }), [], [mapToken('c-toren', 'Toren')]));
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
    expect(Array.from(el.querySelectorAll('button'), (b) => b.textContent?.trim())).toContain('Voltar ao seu mapa');
  });

  it('says "à sua vista" in the map editor', async () => {
    const fixture = await render('à sua vista');
    const el = fixture.nativeElement as HTMLElement;
    const backs: number[] = [];
    fixture.componentInstance.back.subscribe(() => backs.push(1));
    expect(text(el)).toContain('Você está vendo o mapa como Toren');
    expect(text(el)).toContain('Para voltar à sua vista, escolha “Todos”.');
    const button = Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.trim() === 'Voltar à sua vista')!;
    button.click();
    expect(backs).toEqual([1]);
  });
});
