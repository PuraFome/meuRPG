import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { MapsClient } from '../../../core/maps/maps-client';
import { FakeMapsClient, mapMessage } from '../../../core/maps/maps-testing';
import { MapsPanel, mapRowSub } from './maps-panel';
import { nextTurn } from '../../../../test-settle';

async function render(isMaster: boolean, maps: FakeMapsClient): Promise<HTMLElement> {
  TestBed.configureTestingModule({
    providers: [provideRouter([]), { provide: MapsClient, useValue: maps }],
  });
  const fixture = TestBed.createComponent(MapsPanel);
  fixture.componentRef.setInput('campaignId', 'camp-1');
  fixture.componentRef.setInput('isMaster', isMaster);
  fixture.detectChanges();
  await fixture.whenStable();
  await nextTurn();
  fixture.detectChanges();
  return fixture.nativeElement;
}

describe('MapsPanel', () => {
  it('invites the master to create the first map', async () => {
    const el = await render(true, new FakeMapsClient());
    expect(el.textContent).toContain(
      'Nenhum mapa ainda. Crie o primeiro a partir de uma imagem da galeria.',
    );
    expect(el.querySelector('a')?.textContent).toContain('Novo mapa');
  });

  it('lists the maps with their tags for the master', async () => {
    const maps = new FakeMapsClient();
    maps.maps = [
      mapMessage('a', 'Mirathel e arredores', { revealed: true, current: true, pointCount: 5 }),
      mapMessage('b', 'Torre de Mirathel', {
        revealed: false,
        parentMaps: [{ id: 'a', name: 'Mirathel e arredores' } as never],
        pointCount: 1,
      }),
    ];
    const el = await render(true, maps);
    const rows = el.querySelectorAll('li');
    expect(rows[0].textContent).toContain('5 pontos');
    expect(rows[0].textContent).toContain('Mapa atual');
    expect(rows[0].textContent).toContain('Revelado');
    expect(rows[1].textContent).toContain('Submapa de Mirathel e arredores · 1 ponto');
    expect(rows[1].textContent).toContain('Escondido');
  });

  it('gives the master "Gerar masmorra" and "Gerar tesouro", next to "Novo mapa", on the maps list (MR-010, MR-044)', async () => {
    const el = await render(true, new FakeMapsClient());
    const links = Array.from(el.querySelectorAll('app-panel-head a'));
    expect(
      links.map((a) =>
        a.textContent?.replace(a.querySelector('mat-icon')?.textContent ?? '', '').trim(),
      ),
    ).toEqual(['Novo mapa', 'Gerar masmorra', 'Gerar tesouro']);
    expect(links[1]!.getAttribute('href')).toBe('/campaigns/camp-1/maps/dungeon');
    expect(links[2]!.getAttribute('href')).toBe('/campaigns/camp-1/treasure');
  });

  it('gives a player no panel while there is no map, and no "Novo mapa" ever', async () => {
    expect((await render(false, new FakeMapsClient())).querySelector('section')).toBeNull();
  });

  it('gives a player the maps they may open, without the master tags or point counts', async () => {
    TestBed.resetTestingModule();
    const maps = new FakeMapsClient();
    maps.maps = [mapMessage('a', 'Mirathel e arredores', { revealed: true, pointCount: 5 })];
    const el = await render(false, maps);
    expect(el.textContent).toContain('Mirathel e arredores');
    expect(el.textContent).not.toContain('5 pontos');
    expect(el.textContent).not.toContain('Revelado');
    expect(el.textContent).not.toContain('Novo mapa');
    expect(el.textContent).not.toContain('Gerar masmorra');
  });

  it('describes a map by where it sits, its squares and its points', () => {
    expect(mapRowSub(mapMessage('a', 'A', { pointCount: 0 }), true)).toBe('0 pontos');
    expect(
      mapRowSub(
        mapMessage('b', 'B', {
          pointCount: 3,
          gridColumns: 24,
          gridRows: 16,
          parentMaps: [{ id: 'a', name: 'A' } as never],
        }),
        true,
      ),
    ).toBe('Submapa de A · 24 × 16 quadrados · 3 pontos');
    expect(mapRowSub(mapMessage('a', 'A', {}), false)).toBe('Mapa principal');
  });

  it('counts the maps in the title row, shows five and "Ver todos os 7 mapas", which shows the rest', async () => {
    const maps = new FakeMapsClient();
    maps.maps = Array.from({ length: 7 }, (_, i) => mapMessage(`m${i}`, `Mapa ${i}`, {}));
    const el = await render(true, maps);
    expect(el.querySelector('app-panel-head')?.textContent).toContain('7 mapas');
    expect(el.querySelectorAll('li')).toHaveLength(5);
    const more = Array.from(el.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('Ver todos os 7 mapas'),
    )!;
    more.click();
    await nextTurn();
    expect(el.querySelectorAll('li')).toHaveLength(7);
  });
});
