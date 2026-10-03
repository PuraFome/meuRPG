import { ApplicationRef, Injectable, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { BehaviorSubject } from 'rxjs';

import { AuthService } from '../../core/auth/auth.service';
import { MapsClient } from '../../core/maps/maps-client';
import { RosterClient } from '../../core/maps/roster-client';
import { ProgressionClient } from '../../core/progression/progression-client';
import { XpChanges } from '../../core/progression/xp-changes';
import { create } from '@bufbuild/protobuf';
import { XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CharacterExperienceSchema, GetCampaignExperienceResponseSchema } from '../../../gen/meurpg/progression/v1/progression_pb';
import { FakeMapsClient, mapMessage, mapPoint, mapResponse, mapToken } from '../../core/maps/maps-testing';
import { SceneClient } from '../../core/play/scene-client';
import { FakeSceneClient, masterScene, playerScene, sceneRoll } from '../../core/play/scene-testing';
import { SceneActionSchema } from '../../../gen/meurpg/maps/v1/maps_pb';
import { OpenSessions } from '../../shell/live-notice/open-sessions';
import { LiveSession } from './live-session';
import {
  CampaignInfoVm,
  LiveErrorKind,
  LiveEventVm,
  LiveSessionSource,
  LiveSnapshotVm,
  PartyMemberInfoVm,
  PlayerSheetVm,
  ShownImageVm,
} from './live-session.types';
import { brisaVitals, pensantusVitals } from './testing';

class KindError extends Error {
  constructor(readonly kind: LiveErrorKind) {
    super(kind);
  }
}

/** A `LiveSessionSource` whose stream the test feeds by hand. */
@Injectable()
class FakeLiveSessionSource implements LiveSessionSource {
  campaign: CampaignInfoVm | Error = { name: 'Mirathel', isMaster: false, awaitingApproval: false, diceMode: 1, dicePreference: 1 };
  snapshot: LiveSnapshotVm | Error = {
    session: { sessionId: 's4', sessionNumber: 4, startedAt: new Date(2026, 8, 30, 20, 5) },
    vitals: [pensantusVitals()],
    currentMapId: null,
    shownImage: null,
    shownImageKeep: false,
  };
  sheet: PlayerSheetVm = { armorClass: 14, summary: 'Mago 3, Gnomo das Rochas' };
  party = new Map<string, PartyMemberInfoVm>([
    ['pensantus', { classSummary: 'Mago 3', playerName: 'Vinicius' }],
    ['brisa', { classSummary: 'Ladina 3', playerName: 'Ana' }],
  ]);
  /** What the next `watch` call does: yields these, then fails or waits. */
  events: LiveEventVm[] = [{ kind: 'ready' }];
  failWith: Error | null = null;
  readonly endSession = vi.fn(() => Promise.resolve());
  readonly adjustVitals = vi.fn();
  readonly setCurrentMap = vi.fn((_c: string, mapId: string | null) => Promise.resolve(mapId));
  readonly setShownImage = vi.fn();
  /** The images left with the players, as the server lists them. */
  left: ShownImageVm[] = [];
  readonly listLeftImages = vi.fn(() => Promise.resolve(this.left));
  readonly takeBackLeftImage = vi.fn((_c: string, id: string) => {
    this.left = this.left.filter((i) => i.id !== id);
    return Promise.resolve();
  });

  getCampaign(): Promise<CampaignInfoVm> {
    return this.campaign instanceof Error
      ? Promise.reject(this.campaign)
      : Promise.resolve(this.campaign);
  }

  private later: ((e: LiveEventVm) => void) | null = null;

  /** Sends one more event on the open stream. */
  push(event: LiveEventVm): void {
    this.later?.(event);
  }

  async *watch(_campaignId: string, signal: AbortSignal): AsyncIterable<LiveEventVm> {
    for (const e of this.events) {
      yield e;
    }
    if (this.failWith) {
      throw this.failWith;
    }
    // Then stay open, taking `push`ed events, until the page closes it.
    for (;;) {
      const next = await new Promise<LiveEventVm>((resolve, reject) => {
        this.later = resolve;
        signal.addEventListener('abort', () => reject(new Error('aborted')));
      });
      yield next;
    }
  }

  getLiveSession(): Promise<LiveSnapshotVm> {
    return this.snapshot instanceof Error
      ? Promise.reject(this.snapshot)
      : Promise.resolve(this.snapshot);
  }

  getPlayerSheet(): Promise<PlayerSheetVm> {
    return Promise.resolve(this.sheet);
  }

  getPartyInfo(): Promise<ReadonlyMap<string, PartyMemberInfoVm>> {
    return Promise.resolve(this.party);
  }

  classifyError(err: unknown): LiveErrorKind {
    return err instanceof KindError ? err.kind : 'transient';
  }
}

describe('LiveSession', () => {
  let source: FakeLiveSessionSource;
  /** What the master's "Dar XP" reads (MR-016): the party's XP and how the campaign levels. */
  const xpExperience = vi.fn();
  let scenes: FakeSceneClient;
  const signIn = vi.fn();
  const liveCampaignIds = signal<ReadonlySet<string>>(new Set());
  const openSessions = {
    liveCampaignIds: liveCampaignIds.asReadonly(),
    refresh: vi.fn(() => Promise.resolve()),
    dismiss: vi.fn(),
  };

  beforeEach(() => {
    xpExperience.mockReset().mockResolvedValue(
      create(GetCampaignExperienceResponseSchema, {
        xpMode: XpMode.ENEMIES,
        characters: [create(CharacterExperienceSchema, { characterId: 'pensantus', name: 'Pensantus', level: 3, experiencePoints: 2600, nextLevelXp: 2700 })],
      }),
    );
    signIn.mockClear();
    openSessions.dismiss.mockClear();
    liveCampaignIds.set(new Set());
    scenes = new FakeSceneClient();
    TestBed.configureTestingModule({
      imports: [LiveSession],
      providers: [
        provideRouter([]),
        { provide: LiveSessionSource, useClass: FakeLiveSessionSource },
        { provide: MapsClient, useClass: FakeMapsClient },
        { provide: ProgressionClient, useValue: { experience: xpExperience, listAwards: vi.fn() } },
        { provide: RosterClient, useValue: { list: () => Promise.resolve([]) } },
        { provide: SceneClient, useValue: scenes },
        { provide: AuthService, useValue: { signIn, state: signal({ status: 'signed-in' }) } },
        { provide: OpenSessions, useValue: openSessions },
        {
          provide: ActivatedRoute,
          useValue: { paramMap: new BehaviorSubject(convertToParamMap({ id: 'mirathel' })) },
        },
      ],
    });
    source = TestBed.inject(LiveSessionSource) as unknown as FakeLiveSessionSource;
  });

  async function settle(fixture: ComponentFixture<LiveSession>): Promise<HTMLElement> {
    fixture.detectChanges();
    for (let i = 0; i < 4; i++) {
      await fixture.whenStable();
      await new Promise((r) => setTimeout(r));
      fixture.detectChanges();
    }
    return fixture.nativeElement as HTMLElement;
  }

  function render(): Promise<HTMLElement> {
    return settle(TestBed.createComponent(LiveSession));
  }

  function button(el: HTMLElement, name: string): HTMLButtonElement {
    return Array.from(el.querySelectorAll('button')).find((b) =>
      b.textContent?.includes(name),
    ) as HTMLButtonElement;
  }

  it("shows a player their own character's vitals, with the CA from the sheet (E5-02)", async () => {
    const el = await render();
    expect(el.querySelector('h1')?.textContent).toContain('Sessão 4');
    expect(el.textContent).toContain('Mirathel');
    expect(el.textContent).toContain('Ao vivo');
    expect(el.textContent).toContain('Em andamento desde 30/09 às 20:05');
    expect(el.querySelector('.hp__current')?.textContent).toBe('17');
    expect(el.textContent).toContain('de 23');
    expect(el.querySelector('.shield__number')?.textContent).toBe('14');
    expect(el.textContent).toContain('Mago 3, Gnomo das Rochas');
    expect(el.textContent).toContain('1 de 3 usados');
    expect(el.querySelector('[aria-label="1º círculo: 2 livres de 4"]')).not.toBeNull();
    expect(el.textContent).toContain('O mestre ainda não escolheu um mapa.');
    expect(el.textContent).not.toContain('Ajustar');
    // Here already: the notice about this session is spent.
    expect(openSessions.dismiss).toHaveBeenCalledWith('s4');
  });

  describe('XP (MR-016)', () => {
    const asMaster = () => {
      source.campaign = { name: 'Mirathel', isMaster: true, awaitingApproval: false, diceMode: 1, dicePreference: 1 };
    };

    it('gives the master "Dar XP" in the party panel', async () => {
      asMaster();
      const master = await render();
      expect(master.querySelector('app-party-panel app-xp-give-button')).not.toBeNull();
      expect(button(master, 'Dar XP')).toBeTruthy();
    });

    it('says "Registrar marco" instead in a campaign that levels by milestones', async () => {
      asMaster();
      xpExperience.mockResolvedValue(create(GetCampaignExperienceResponseSchema, { xpMode: XpMode.MILESTONES, characters: [] }));
      const el = await render();
      expect(button(el, 'Registrar marco')).toBeTruthy();
      expect(button(el, 'Dar XP')).toBeUndefined();
    });

    it('does not read the XP for a player (only the master has "Dar XP" here)', async () => {
      await render();
      expect(xpExperience).not.toHaveBeenCalled();
    });

    it('lets everything that depends on XP know when the stream says it changed, and ignores what it does not know', async () => {
      asMaster();
      await render();
      const changes = TestBed.inject(XpChanges);
      const before = changes.version();
      xpExperience.mockClear();

      source.push({ kind: 'xpChanged' });
      await new Promise((r) => setTimeout(r));
      expect(changes.version()).toBe(before + 1);
      // The party panel's "Dar XP" reads the characters again.
      await new Promise((r) => setTimeout(r));
      expect(xpExperience).toHaveBeenCalledTimes(1);
    });
  });

  it('applies a vitals change from the stream without a reload', async () => {
    source.events = [
      { kind: 'ready' },
      { kind: 'vitals', vitals: pensantusVitals({ revision: 2, hitPointsCurrent: 12 }) },
    ];
    const el = await render();
    expect(el.querySelector('.hp__current')?.textContent).toBe('12');
  });

  it('shows the master the party with "Ajustar" per character, and the session link (E5-04)', async () => {
    source.campaign = { name: 'Mirathel', isMaster: true, awaitingApproval: false, diceMode: 1, dicePreference: 1 };
    source.snapshot = {
      session: { sessionId: 's4', sessionNumber: 4, startedAt: new Date(2026, 8, 30, 20, 5) },
      vitals: [pensantusVitals(), brisaVitals()],
      currentMapId: null,
      shownImage: null,
      shownImageKeep: false,
    };
    const el = await render();
    expect(el.textContent).toContain('Em andamento desde 30/09 às 20:05');
    expect(el.querySelector('[aria-label="Ajustar Pensantus"]')).not.toBeNull();
    expect(el.querySelector('[aria-label="Ajustar Brisa"]')).not.toBeNull();
    expect(el.textContent).toContain('Ladina 3, de Ana');
    expect(el.textContent).toContain('+5 temporários');
    expect(el.textContent).toContain('Abaixo da metade');
    expect(el.querySelector<HTMLInputElement>('#session-link')?.value).toBe(
      `${location.origin}/campanhas/mirathel/sessao`,
    );
    expect(el.textContent).toContain('Copiar link da sessão');
    expect(el.textContent).toContain('Encerrar sessão');
  });

  it('says "Peça um convite ao mestre", without the name, to a non-member', async () => {
    source.campaign = new KindError('no-access');
    const el = await render();
    expect(el.querySelector('h1')?.textContent).toContain('Peça um convite ao mestre');
    expect(el.textContent).not.toContain('Mirathel');
  });

  it('says the same to a pending member (RN-15)', async () => {
    source.campaign = { name: 'Mirathel', isMaster: false, awaitingApproval: true, diceMode: 1, dicePreference: 1 };
    const el = await render();
    expect(el.querySelector('h1')?.textContent).toContain('Peça um convite ao mestre');
    expect(el.textContent).not.toContain('Mirathel');
  });

  it('says "Nenhuma sessão em andamento", with the name, when none is open', async () => {
    source.events = [];
    source.failWith = new KindError('no-session');
    const el = await render();
    expect(el.querySelector('h1')?.textContent).toContain('Nenhuma sessão em andamento');
    expect(el.textContent).toContain('Mirathel');
    expect(el.textContent).toContain('Voltar para a campanha');
  });

  it('shows "Sessão 4 encerrada" when the stream says the session ended', async () => {
    const fixture = TestBed.createComponent(LiveSession);
    const el = await settle(fixture);
    expect(el.querySelector('h1')?.textContent).toContain('Sessão 4');

    source.push({ kind: 'ended' });
    await settle(fixture);
    expect(el.querySelector('h1')?.textContent).toContain('Sessão 4 encerrada');
    expect(el.textContent).toContain('A sessão acabou.');
    expect(openSessions.refresh).toHaveBeenCalled();
  });

  it('the master ends the session after confirming in place', async () => {
    source.campaign = { name: 'Mirathel', isMaster: true, awaitingApproval: false, diceMode: 1, dicePreference: 1 };
    const fixture = TestBed.createComponent(LiveSession);
    const el = await settle(fixture);

    button(el, 'Encerrar sessão').click();
    await settle(fixture);
    expect(source.endSession).not.toHaveBeenCalled();
    button(el, 'Confirmar encerramento').click();
    await settle(fixture);

    expect(source.endSession).toHaveBeenCalledWith('mirathel', 's4');
    expect(el.querySelector('h1')?.textContent).toContain('Sessão 4 encerrada');
  });

  it('opens the session by itself when the master starts it (the RN-06 poll sees it)', async () => {
    source.events = [];
    source.failWith = new KindError('no-session');
    const fixture = TestBed.createComponent(LiveSession);
    const el = await settle(fixture);
    expect(el.querySelector('h1')?.textContent).toContain('Nenhuma sessão em andamento');

    source.events = [{ kind: 'ready' }];
    source.failWith = null;
    liveCampaignIds.set(new Set(['mirathel']));
    await settle(fixture);
    expect(el.querySelector('h1')?.textContent).toContain('Sessão 4');
    expect(el.querySelector('.hp__current')?.textContent).toBe('17');
  });

  it('sends a signed-out person to sign in and back', async () => {
    source.events = [];
    source.failWith = new KindError('signed-out');
    await render();
    expect(signIn).toHaveBeenCalled();
  });

  describe('the map and the image on show (MR-012, MR-028)', () => {
    let maps: FakeMapsClient;

    beforeEach(() => {
      maps = TestBed.inject(MapsClient) as unknown as FakeMapsClient;
      const map = mapMessage('map-1', 'Mirathel e arredores', { revealed: true, current: true });
      maps.responses.set(
        'map-1',
        mapResponse(
          map,
          [mapPoint('p1', 'Taverna do Javali', { revealed: true })],
          [mapToken('pensantus', 'Pensantus', { mine: true, xBp: 5200, yBp: 5400 })],
        ),
      );
      maps.maps = [map];
      source.snapshot = {
        ...(source.snapshot as LiveSnapshotVm),
        currentMapId: 'map-1',
      };
    });

    it('draws the current map from the snapshot, with the party and a link to the full map', async () => {
      const el = await render();
      expect(el.querySelector('#session-map-heading')?.textContent).toContain('Mirathel e arredores');
      expect(maps.calls).toContain('get map-1');
      expect(el.querySelector('[role="img"][aria-label="Prévia do mapa Mirathel e arredores"]')).not.toBeNull();
      expect(el.textContent).toContain('Pensantus');
      expect(el.textContent).toContain('(você)');
      expect(el.textContent).toContain('Ver mapa');
      expect(el.textContent).not.toContain('O mestre ainda não escolheu um mapa.');
    });

    it('moves a token from the stream without reading the map again', async () => {
      const el = await render();
      const gets = maps.calls.filter((c) => c.startsWith('get')).length;
      source.push({ kind: 'tokenMoved', mapId: 'map-1', characterId: 'pensantus', xBp: 6200, yBp: 5400 });
      await new Promise((r) => setTimeout(r));
      TestBed.inject(ApplicationRef).tick();
      expect(el.querySelector('app-map-token')?.getAttribute('style')).toContain('left: 62%');
      expect(maps.calls.filter((c) => c.startsWith('get')).length).toBe(gets);
    });

    it('reads the map again on map_changed, and leaves it when the player lost sight of it', async () => {
      const el = await render();
      const gets = maps.calls.filter((c) => c.startsWith('get')).length;
      source.push({ kind: 'mapChanged', mapId: 'map-1' });
      await new Promise((r) => setTimeout(r));
      expect(maps.calls.filter((c) => c.startsWith('get')).length).toBe(gets + 1);

      maps.responses.delete('map-1'); // not_found: the map is hidden from them now
      source.push({ kind: 'mapChanged', mapId: 'map-1' });
      await new Promise((r) => setTimeout(r));
      TestBed.inject(ApplicationRef).tick();
      expect(el.textContent).toContain('O mestre ainda não escolheu um mapa.');
    });

    it('follows current_map_changed to another map, or to none', async () => {
      const el = await render();
      const other = mapMessage('map-2', 'Torre de Mirathel', { revealed: true, current: true });
      maps.responses.set('map-2', mapResponse(other));
      source.push({ kind: 'currentMap', mapId: 'map-2' });
      await new Promise((r) => setTimeout(r));
      TestBed.inject(ApplicationRef).tick();
      expect(el.querySelector('#session-map-heading')?.textContent).toContain('Torre de Mirathel');
      source.push({ kind: 'currentMap', mapId: null });
      await new Promise((r) => setTimeout(r));
      TestBed.inject(ApplicationRef).tick();
      expect(el.textContent).toContain('O mestre ainda não escolheu um mapa.');
    });

    it('shows the player the image on show, announces it, and takes it away when it stops', async () => {
      const el = await render();
      expect(el.textContent).not.toContain('O mestre está mostrando');
      const image = { id: 'img-1', name: 'Capitão Goblin', width: 400, height: 500, url: '/images/img-1' };
      source.push({ kind: 'shownImage', image });
      await new Promise((r) => setTimeout(r));
      TestBed.inject(ApplicationRef).tick();
      expect(el.querySelector('#shown-title')?.textContent).toContain('O mestre está mostrando');
      expect(el.querySelector('.block__name')?.textContent).toContain('Capitão Goblin');
      expect(el.querySelector('img[alt="Capitão Goblin"]')?.getAttribute('src')).toBe('/images/img-1');
      expect(el.querySelector('[role="status"]:not(.status)')).not.toBeNull();
      expect(el.textContent).toContain('O mestre está mostrando Capitão Goblin.');

      source.push({ kind: 'shownImage', image: null });
      await new Promise((r) => setTimeout(r));
      TestBed.inject(ApplicationRef).tick();
      expect(el.textContent).toContain('O mestre parou de mostrar a imagem.');
    });

    it('shows the player the images the master left, live, with no announcement (E6-25b)', async () => {
      const tower = { id: 'img-2', name: 'Planta da torre', width: 800, height: 600, url: '/images/img-2' };
      source.left = [tower];
      const el = await render();
      expect(el.querySelector('#left-title')).not.toBeNull();
      expect(el.querySelector('.row__name')?.textContent).toContain('Planta da torre');
      expect(el.querySelector('.row__thumb')?.getAttribute('src')).toBe('/images/img-2/thumb');
      expect(el.querySelector('button[aria-label="Ver Planta da torre em tela cheia"]')).not.toBeNull();
      expect(el.querySelector('.block__note')?.textContent).toContain('Ficam aqui até o mestre tirar.');

      source.left = [];
      source.push({ kind: 'leftImages' });
      await new Promise((r) => setTimeout(r));
      TestBed.inject(ApplicationRef).tick();
      expect(el.querySelector('#left-title')).toBeNull();
      expect(el.querySelector('[role="status"]:not(.status)')?.textContent?.trim() ?? '').toBe('');
    });

    it('draws the block without announcing it when it comes with the snapshot (a reload)', async () => {
      source.snapshot = {
        ...(source.snapshot as LiveSnapshotVm),
        shownImage: { id: 'img-1', name: 'Carta', width: 800, height: 500, url: '/images/img-1' },
      };
      const el = await render();
      expect(el.querySelector('.block__name')?.textContent).toContain('Carta');
      expect(el.textContent).not.toContain('O mestre está mostrando Carta.');
    });
  });

  describe('RP scene (MR-015, E7-02, E7-03)', () => {
    async function tick(): Promise<void> {
      await new Promise((r) => setTimeout(r));
      TestBed.inject(ApplicationRef).tick();
    }

    it('draws a scene that was already open with the snapshot, with no announcement and no focus move', async () => {
      scenes.scene = playerScene();
      const el = await render();
      expect(el.querySelector('#sc-title')?.textContent).toBe('Cena: A carroça tombada');
      expect(el.textContent).not.toContain('O mestre abriu uma cena: A carroça tombada.');
      expect(document.activeElement).not.toBe(el.querySelector('#sc-title'));
      expect(scenes.calls).toContain('get');
    });

    it('shows nothing extra while no scene is open', async () => {
      const el = await render();
      expect(el.querySelector('app-scene-player')?.textContent?.trim()).toBe('');
      expect(el.textContent).not.toContain('Nenhuma cena');
    });

    it('reads the scene again on scene_changed, announces the title, and says when the master closes it', async () => {
      const el = await render();
      scenes.scene = playerScene();
      source.push({ kind: 'sceneChanged' });
      await tick();
      await tick();
      expect(el.querySelector('#sc-title')?.textContent).toBe('Cena: A carroça tombada');
      expect(el.textContent).toContain('O mestre abriu uma cena: A carroça tombada.');
      // The page does not take focus from where the player is.
      expect(document.activeElement).not.toBe(el.querySelector('#sc-title'));

      scenes.scene = null;
      source.push({ kind: 'sceneChanged' });
      await tick();
      await tick();
      expect(el.querySelector('#sc-title')).toBeNull();
      expect(el.textContent).toContain('O mestre fechou a cena.');
    });

    it("turns the player's row into the result when scene_check_rolled arrives for their own roll", async () => {
      scenes.scene = playerScene();
      const el = await render();
      expect(el.querySelectorAll('.sc__roll')).toHaveLength(5);
      scenes.scene = playerScene([sceneRoll('r1', 'a1', 'Pensantus', 17)]);
      source.push({ kind: 'sceneCheckRolled' });
      await tick();
      await tick();
      expect(el.querySelectorAll('.sc__roll')).toHaveLength(4);
      expect(el.querySelector('.sc__done')?.textContent).toContain('Rolada');
    });

    describe('the master', () => {
      beforeEach(() => {
        source.campaign = { name: 'Mirathel', isMaster: true, awaitingApproval: false, diceMode: 1, dicePreference: 1 };
        source.snapshot = {
          session: { sessionId: 's4', sessionNumber: 4, startedAt: new Date(2026, 8, 30, 20, 5) },
          vitals: [pensantusVitals(), brisaVitals()],
          currentMapId: 'map-1',
          shownImage: null,
          shownImageKeep: false,
        };
        const maps = TestBed.inject(MapsClient) as unknown as FakeMapsClient;
        maps.responses.set(
          'map-1',
          mapResponse(mapMessage('map-1', 'Estrada do Vale', { revealed: true, current: true }), [mapPoint('p1', 'A carroça tombada')]),
        );
      });

      it('has "Cena de RP" with "Abrir cena" while none is open, and no open-scene block', async () => {
        const el = await render();
        expect(el.querySelector('app-scene-panel h2')?.textContent).toBe('Cena de RP');
        expect(el.textContent).toContain('Nenhuma cena aberta.');
        expect(el.querySelector('app-scene-open')).toBeNull();
        expect(el.querySelector('.board--scene-open')).toBeNull();
      });

      it('moves the open scene to the top of the map column and takes "Cena de RP" out of the right one', async () => {
        scenes.scene = masterScene([sceneRoll('r1', 'a2', 'Toren', 7, { passed: false })]);
        const el = await render();
        expect(el.querySelector('app-scene-panel')).toBeNull();
        expect(el.querySelector('.board--scene-open')).not.toBeNull();
        const left = el.querySelector('.board__left')!;
        expect(left.firstElementChild?.tagName.toLowerCase()).toBe('app-scene-open');
        expect(left.querySelector('app-session-map')).not.toBeNull();
        expect(el.querySelector('#so-title')?.textContent).toBe('Cena: A carroça tombada');
        expect(el.textContent).toContain('Não passou');
      });

      it('reads the rolls again on scene_check_rolled and reads the new one aloud', async () => {
        scenes.scene = masterScene();
        const el = await render();
        expect(el.textContent).toContain('Ninguém rolou ainda.');
        scenes.scene = masterScene([sceneRoll('r1', 'a2', 'Toren', 7, { passed: false })]);
        source.push({ kind: 'sceneCheckRolled' });
        await tick();
        await tick();
        expect(el.textContent).toContain('Toren: Seguir os rastros dos goblins, 7, não passou');
        expect(el.querySelectorAll('app-scene-roll-line')).toHaveLength(1);
      });

      it('opens a scene from its point in "Pontos do mapa", and the list says it is the open one', async () => {
        const maps = TestBed.inject(MapsClient) as unknown as FakeMapsClient;
        maps.responses.set(
          'map-1',
          mapResponse(mapMessage('map-1', 'Estrada do Vale', { revealed: true, current: true }), [
            mapPoint('p1', 'A carroça tombada', { revealed: true, sceneActions: [create(SceneActionSchema, { id: 'a1', key: 'skill:arcana' })] }),
          ]),
        );
        scenes.scene = masterScene();
        const el = await render();
        // A scene is already open: the point says so.
        expect(el.querySelector('.row__open')?.textContent).toContain('Cena aberta agora');
        scenes.scene = null;
        source.push({ kind: 'sceneChanged' });
        await tick();
        await tick();
        const open = el.querySelector<HTMLButtonElement>('button[aria-label="Abrir cena A carroça tombada"]')!;
        scenes.scene = masterScene();
        open.click();
        await tick();
        await tick();
        expect(scenes.calls).toContain('open p1');
        expect(el.querySelector('app-scene-open')).not.toBeNull();
        expect(document.activeElement).toBe(el.querySelector('#so-title'));
      });

      it('brings "Cena de RP" back when the scene closes from another tab', async () => {
        scenes.scene = masterScene();
        const el = await render();
        scenes.scene = null;
        source.push({ kind: 'sceneChanged' });
        await tick();
        await tick();
        expect(el.querySelector('app-scene-open')).toBeNull();
        expect(el.querySelector('app-scene-panel')).not.toBeNull();
      });
    });
  });
});
