import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { provideRouter } from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';

import { MapPointKind } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { flat, isOff } from '../../../core/creatures/creatures-testing';
import { EncountersClient } from '../../../core/encounters/encounters-client';
import { BUGBEAR, FakeEncountersClient, GOBLIN, HOBGOBLIN, OGRE, evaluation, line } from '../../../core/encounters/encounters-testing';
import { MapsClient } from '../../../core/maps/maps-client';
import { SaveSheet, type SaveData } from './save-sheet';

describe('SaveSheet: "Guardar no ponto de batalha" (MR-043, E10-09 state 6)', () => {
  let api: FakeEncountersClient;
  let close: ReturnType<typeof vi.fn>;
  let points: { id: string; name: string; kind: MapPointKind }[];
  let mapsFail: unknown;

  async function setup(over: Partial<SaveData> = {}) {
    api = new FakeEncountersClient();
    api.kept = [{ mapPointId: 'pt-2', creatureCount: 7 }];
    close = vi.fn();
    points = [
      { id: 'pt-1', name: 'Emboscada na ponte', kind: MapPointKind.BATTLE },
      { id: 'pt-2', name: 'Ruínas do forte', kind: MapPointKind.BATTLE },
      { id: 'pt-3', name: 'Taverna', kind: MapPointKind.SCENE },
    ];
    mapsFail = null;
    const maps = {
      list: vi.fn(async () => {
        if (mapsFail) {
          throw mapsFail;
        }
        return [{ id: 'map-1', name: 'Estrada do Vale' }, { id: 'map-2', name: 'Masmorra' }];
      }),
      get: vi.fn(async (_c: string, mapId: string) => ({ points: mapId === 'map-1' ? points : [] })),
    };
    const entries = [OGRE, BUGBEAR, HOBGOBLIN, GOBLIN].map((c, i) => ({ creatureKey: c.key, count: [1, 2, 4, 6][i] }));
    const data: SaveData = {
      campaignId: 'camp-1',
      entries,
      party: [{ characterId: 'npc-1', name: '', level: 3 }],
      evaluation: evaluation([line(OGRE, 1), line(BUGBEAR, 2), line(HOBGOBLIN, 4), line(GOBLIN, 6)]),
      ...over,
    };
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: EncountersClient, useValue: api },
        { provide: MapsClient, useValue: maps },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close } },
      ],
    });
    const fixture = TestBed.createComponent(SaveSheet);
    const settle = async () => {
      for (let i = 0; i < 5; i++) {
        fixture.detectChanges();
        await fixture.whenStable();
      }
    };
    await settle();
    const el = fixture.nativeElement as HTMLElement;
    const button = (name: string) => Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) => (flat(b) ?? '').includes(name))!;
    const pick = (name: string) => Array.from(el.querySelectorAll<HTMLLabelElement>('.pt')).find((l) => flat(l)?.includes(name))!.querySelector('input')!;
    return { el, settle, button, pick, maps };
  }

  it('lists the map and its battle points (only battle ones), says which already keeps an encounter, and waits for a choice', async () => {
    const { el, button } = await setup();
    expect(flat(el.querySelector('.frame__title'))).toBe('Guardar no ponto de batalha');
    expect(flat(el.querySelector('.frame__sub'))).toBe('Moderada · 1.550 de 1.875 XP · 13 criaturas');
    expect(el.querySelector<HTMLSelectElement>('select[name=map]')!.value).toBe('map-1');
    expect(Array.from(el.querySelectorAll('.pt')).map((p) => flat(p))).toEqual([
      'Emboscada na ponte Sem encontro ainda',
      'Ruínas do forte Já guarda um encontro (7 criaturas): guardar de novo troca o anterior',
    ]);
    expect(flat(el.querySelector('.why'))).toBe('Escolha um ponto de batalha.');
    expect(isOff(button('Guardar no ponto de batalha'))).toBe(true);
    expect(flat(el.querySelector('.priv'))).toBe('Só você vê O encontro, as criaturas e o XP ficam escondidos dos jogadores até você revelá-los no combate.');
    expect(flat(el.querySelector('.guide'))).toBe('Guia de dificuldade do SRD 5.2.1 (regras de 2024) · Créditos');
  });

  it('keeps the encounter on a free point: the creatures, the average, hidden, and the party\'s NPCs, then closes with where', async () => {
    const { settle, button, pick } = await setup();
    pick('Emboscada na ponte').click();
    await settle();
    button('Guardar no ponto de batalha').click();
    await settle();
    expect(api.saves).toHaveLength(1);
    expect(api.saves[0]).toMatchObject({
      pointId: 'pt-1',
      encounter: { hp: 'average', hidden: true, monsters: [{ creatureKey: 'monster:ogre', count: 1 }, { creatureKey: 'monster:bugbear', count: 2 }, { creatureKey: 'monster:hobgoblin', count: 4 }, { creatureKey: 'monster:goblin', count: 6 }] },
      party: [{ characterId: 'npc-1', name: '', level: 3 }],
    });
    expect(close).toHaveBeenCalledWith(expect.objectContaining({ mapId: 'map-1', pointName: 'Emboscada na ponte' }));
  });

  it('a point that keeps one asks in place before it replaces, with the focus on "Voltar"; "Voltar" saves nothing', async () => {
    const { el, settle, button, pick } = await setup();
    pick('Ruínas do forte').click();
    await settle();
    button('Guardar no ponto de batalha').click();
    await settle();
    expect(api.saves).toHaveLength(0);
    expect(flat(el.querySelector('.ask'))).toBe('Trocar o encontro guardado? “Ruínas do forte” já guarda um encontro de 7 criaturas. Guardar este põe o novo no lugar dele.');
    // One set of buttons: the footer's, now "Trocar o encontro" and "Voltar".
    expect(Array.from(el.querySelectorAll('button')).filter((b) => flat(b) === 'Voltar')).toHaveLength(1);
    expect(Array.from(el.querySelectorAll('button')).filter((b) => flat(b) === 'Trocar o encontro')).toHaveLength(1);
    expect(el.querySelector('.ask button')).toBeNull();
    expect(document.activeElement?.textContent?.trim()).toBe('Voltar');
    button('Voltar').click();
    await settle();
    expect(el.querySelector('.ask')).toBeNull();
    expect(api.saves).toHaveLength(0);
  });

  it('"Trocar o encontro" replaces it', async () => {
    const { settle, button, pick } = await setup();
    pick('Ruínas do forte').click();
    await settle();
    button('Guardar no ponto de batalha').click();
    await settle();
    button('Trocar o encontro').click();
    await settle();
    expect(api.saves).toHaveLength(1);
    expect(api.saves[0].pointId).toBe('pt-2');
    expect(close).toHaveBeenCalledWith(expect.objectContaining({ pointName: 'Ruínas do forte' }));
  });

  it('opens on the map and the point a link from the editor named', async () => {
    const { el } = await setup({ mapId: 'map-1', pointId: 'pt-1' });
    expect(el.querySelector<HTMLInputElement>('input[name=save-point]:checked')?.value).toBe('pt-1');
    expect(el.querySelector('.why')).toBeNull();
  });

  it('a map with no battle point says so and offers "Abrir o mapa"', async () => {
    const { el, settle } = await setup();
    const select = el.querySelector<HTMLSelectElement>('select[name=map]')!;
    select.value = 'map-2';
    select.dispatchEvent(new Event('change'));
    await settle();
    expect(flat(el.querySelector('.mr-notice'))).toBe('Este mapa não tem ponto de batalha. Ponha um no editor do mapa e volte. Abrir o mapa');
    expect(el.querySelector('.mr-notice a')?.getAttribute('href')).toBe('/campanhas/camp-1/mapas/map-2');
  });

  it('says what a refusal means: the point is gone, or a creature left the SRD', async () => {
    const { el, settle, button, pick } = await setup();
    pick('Emboscada na ponte').click();
    await settle();
    api.saveFail = new ConnectError('gone', Code.NotFound);
    button('Guardar no ponto de batalha').click();
    await settle();
    expect(el.querySelector('[role=alert]')?.textContent).toContain('Esse ponto de batalha não existe mais');
    expect(close).not.toHaveBeenCalled();
  });
});
