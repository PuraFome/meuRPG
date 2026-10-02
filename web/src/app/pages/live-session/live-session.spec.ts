import { ApplicationRef, Injectable, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { BehaviorSubject } from 'rxjs';

import { AuthService } from '../../core/auth/auth.service';
import { MapsClient } from '../../core/maps/maps-client';
import { FakeMapsClient, mapMessage, mapPoint, mapResponse, mapToken } from '../../core/maps/maps-testing';
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
  campaign: CampaignInfoVm | Error = { name: 'Mirathel', isMaster: false, awaitingApproval: false };
  snapshot: LiveSnapshotVm | Error = {
    session: { sessionId: 's4', sessionNumber: 4, startedAt: new Date(2026, 8, 30, 20, 5) },
    vitals: [pensantusVitals()],
    currentMapId: null,
    shownImage: null,
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
  const signIn = vi.fn();
  const liveCampaignIds = signal<ReadonlySet<string>>(new Set());
  const openSessions = {
    liveCampaignIds: liveCampaignIds.asReadonly(),
    refresh: vi.fn(() => Promise.resolve()),
    dismiss: vi.fn(),
  };

  beforeEach(() => {
    signIn.mockClear();
    openSessions.dismiss.mockClear();
    liveCampaignIds.set(new Set());
    TestBed.configureTestingModule({
      imports: [LiveSession],
      providers: [
        provideRouter([]),
        { provide: LiveSessionSource, useClass: FakeLiveSessionSource },
        { provide: MapsClient, useClass: FakeMapsClient },
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
    expect(el.textContent).toContain('atualizado agora');
    expect(el.querySelector('.hp__current')?.textContent).toBe('17');
    expect(el.textContent).toContain('de 23');
    expect(el.querySelector('.shield__number')?.textContent).toBe('14');
    expect(el.textContent).toContain('Mago 3, Gnomo das Rochas');
    expect(el.textContent).toContain('1 de 3 usados');
    expect(el.querySelector('[aria-label="1º círculo: 2 de 4 espaços usados"]')).not.toBeNull();
    expect(el.textContent).toContain('O mestre ainda não escolheu um mapa.');
    expect(el.textContent).not.toContain('Ajustar');
    // Here already: the notice about this session is spent.
    expect(openSessions.dismiss).toHaveBeenCalledWith('s4');
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
    source.campaign = { name: 'Mirathel', isMaster: true, awaitingApproval: false };
    source.snapshot = {
      session: { sessionId: 's4', sessionNumber: 4, startedAt: new Date(2026, 8, 30, 20, 5) },
      vitals: [pensantusVitals(), brisaVitals()],
      currentMapId: null,
      shownImage: null,
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
    source.campaign = { name: 'Mirathel', isMaster: false, awaitingApproval: true };
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
    source.campaign = { name: 'Mirathel', isMaster: true, awaitingApproval: false };
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
});
