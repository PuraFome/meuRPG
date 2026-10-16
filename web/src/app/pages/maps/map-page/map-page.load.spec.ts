import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';
import { MatDialog } from '@angular/material/dialog';
import { ActivatedRoute, Router, convertToParamMap } from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';
import { BehaviorSubject } from 'rxjs';
import { type Mock, afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { Role } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { MapSchema } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { CampaignsService } from '../../../core/campaigns/campaigns.service';
import { CombatOnMap } from '../../../core/maps/combat-on-map';
import { MapsClient } from '../../../core/maps/maps-client';
import { mapMessage, mapResponse } from '../../../core/maps/maps-testing';
import { OpenSessionLookup } from '../../../core/play/open-session';
import { nextTurn } from '../../../../test-settle';
import { MapLiveClient } from './map-live';
import { MapPage } from './map-page';

interface PageView {
  phase(): string;
  isMaster(): boolean;
  campaignId(): string;
  maps(): readonly unknown[];
  busy(): boolean;
  notice(): string | null;
  fromSession(): boolean;
  currentSession(): number | null;
  sessionNumber(): number | null;
  combatRunning(): boolean;
  state: {
    map(): { id: string; name: string; revealed: boolean } | null;
    setMap(m: unknown): void;
  };
  load(campaignId: string, mapId: string): Promise<void>;
  reloadFlags(): void;
  reloadMaps(): Promise<void>;
  retry(): void;
  saveName(name: string): Promise<void>;
  deleteMap(): Promise<void>;
  toggleReveal(): Promise<void>;
  imageGenerated(o: { map?: unknown }): void;
  changeImage(): void;
}

/** A promise the spec settles by hand, to put two loads in flight at once. */
function deferred<T>(): { promise: Promise<T>; resolve(v: T): void; reject(e: unknown): void } {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((a, b) => {
    resolve = a;
    reject = b;
  });
  return { promise, resolve, reject };
}

describe('MapPage, loading the map', () => {
  let api: Record<'get' | 'list' | 'update' | 'delete' | 'setRevealed', Mock>;
  let campaigns: { getCampaign: Mock };
  let combat: { running: Mock };
  let sessions: { currentMap: Mock };
  let watch: Mock;
  let router: { navigate: Mock };
  let dialog: { open: Mock };
  let params: BehaviorSubject<ReturnType<typeof convertToParamMap>>;
  let query: BehaviorSubject<ReturnType<typeof convertToParamMap>>;
  let snapshotMapId: string | null;

  const campaign = (role: Role, awaitingApproval = false) => ({
    campaign: { myRole: role, awaitingApproval },
  });

  function page(): PageView {
    return TestBed.createComponent(MapPage).componentInstance as unknown as PageView;
  }

  beforeEach(() => {
    snapshotMapId = 'm1';
    api = {
      get: vi.fn().mockResolvedValue(mapResponse(mapMessage('m1', 'Cripta'))),
      list: vi.fn().mockResolvedValue([mapMessage('m1', 'Cripta')]),
      update: vi.fn(),
      delete: vi.fn().mockResolvedValue(undefined),
      setRevealed: vi.fn(),
    };
    campaigns = { getCampaign: vi.fn().mockResolvedValue(campaign(Role.MASTER)) };
    combat = { running: vi.fn().mockResolvedValue(false) };
    sessions = { currentMap: vi.fn().mockResolvedValue(null) };
    watch = vi.fn().mockReturnValue({
      [Symbol.asyncIterator]: () => ({ next: () => new Promise(() => undefined) }),
    });
    router = { navigate: vi.fn().mockResolvedValue(true) };
    dialog = { open: vi.fn() };
    params = new BehaviorSubject(convertToParamMap({ id: 'c1', mapId: 'm1' }));
    query = new BehaviorSubject(convertToParamMap({}));
    TestBed.configureTestingModule({
      providers: [
        { provide: MapsClient, useValue: api },
        { provide: CampaignsService, useValue: campaigns },
        { provide: CombatOnMap, useValue: combat },
        { provide: OpenSessionLookup, useValue: sessions },
        { provide: MapLiveClient, useValue: { watch, classifyError: () => 'transient' } },
        { provide: Router, useValue: router },
        { provide: MatDialog, useValue: dialog },
        {
          provide: ActivatedRoute,
          useValue: {
            paramMap: params,
            queryParamMap: query,
            snapshot: {
              paramMap: {
                get: (k: string) => (k === 'mapId' ? snapshotMapId : null),
              },
            },
          },
        },
      ],
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('shows the editor to the master and asks about the open session and the combat', async () => {
    sessions.currentMap.mockResolvedValue({ mapId: 'm1', sessionNumber: 3 });
    combat.running.mockResolvedValue(true);
    const p = page();
    await nextTurn();
    expect(p.phase()).toBe('ready');
    expect(p.isMaster()).toBe(true);
    expect(p.campaignId()).toBe('c1');
    expect(p.currentSession()).toBe(3);
    expect(p.sessionNumber()).toBe(3);
    expect(p.combatRunning()).toBe(true);
    expect(api.list).toHaveBeenCalledWith('c1');
    expect(p.maps()).toHaveLength(1);
    expect(watch).not.toHaveBeenCalled();
  });

  it('keeps the session number but not the map when the open session is on another map', async () => {
    sessions.currentMap.mockResolvedValue({ mapId: 'other', sessionNumber: 4 });
    const p = page();
    await nextTurn();
    expect(p.currentSession()).toBeNull();
    expect(p.sessionNumber()).toBe(4);
  });

  it('follows the table live for a player, and does not ask about the session', async () => {
    campaigns.getCampaign.mockResolvedValue(campaign(Role.PLAYER));
    const p = page();
    await nextTurn();
    expect(p.phase()).toBe('ready');
    expect(p.isMaster()).toBe(false);
    expect(watch).toHaveBeenCalledTimes(1);
    expect(sessions.currentMap).not.toHaveBeenCalled();
  });

  it('shows "gone" for a campaign that still awaits the approval', async () => {
    campaigns.getCampaign.mockResolvedValue(campaign(Role.PLAYER, true));
    const p = page();
    await nextTurn();
    expect(p.phase()).toBe('gone');
    expect(api.get).not.toHaveBeenCalled();
  });

  it('shows "gone" for a map that is not there and "error" for a map it could not read', async () => {
    api.get.mockRejectedValue(new ConnectError('x', Code.NotFound));
    const gone = page();
    await nextTurn();
    expect(gone.phase()).toBe('gone');
    api.get.mockRejectedValue(new ConnectError('x', Code.Unavailable));
    const failed = page();
    await nextTurn();
    expect(failed.phase()).toBe('error');
  });

  it('shows "gone" or "error" when the campaign itself cannot be read', async () => {
    campaigns.getCampaign.mockRejectedValue(new ConnectError('x', Code.NotFound));
    const gone = page();
    await nextTurn();
    expect(gone.phase()).toBe('gone');
    campaigns.getCampaign.mockRejectedValue(new ConnectError('x', Code.Internal));
    const failed = page();
    await nextTurn();
    expect(failed.phase()).toBe('error');
  });

  it('shows "gone" when the map answers with nothing', async () => {
    api.get.mockResolvedValue({ ...mapResponse(mapMessage('m1', 'x')), map: undefined });
    const p = page();
    await nextTurn();
    expect(p.phase()).toBe('gone');
  });

  it('drops the answer of a load that a newer one replaced', async () => {
    const first = deferred<unknown>();
    campaigns.getCampaign.mockReturnValueOnce(first.promise);
    const p = page();
    await nextTurn();
    expect(p.phase()).toBe('loading');
    params.next(convertToParamMap({ id: 'c2', mapId: 'm2' }));
    await nextTurn();
    expect(p.campaignId()).toBe('c2');
    expect(p.phase()).toBe('ready');
    first.resolve(campaign(Role.PLAYER, true));
    await nextTurn();
    expect(p.phase()).toBe('ready');
    expect(p.isMaster()).toBe(true);
  });

  it('drops the error of a load that a newer one replaced', async () => {
    const first = deferred<unknown>();
    campaigns.getCampaign.mockReturnValueOnce(first.promise);
    const p = page();
    await nextTurn();
    params.next(convertToParamMap({ id: 'c2', mapId: 'm2' }));
    await nextTurn();
    first.reject(new ConnectError('x', Code.Internal));
    await nextTurn();
    expect(p.phase()).toBe('ready');
  });

  it('keeps the page on screen, and the campaign, when only the map changes', async () => {
    const p = page();
    await nextTurn();
    campaigns.getCampaign.mockClear();
    params.next(convertToParamMap({ id: 'c1', mapId: 'm2' }));
    expect(p.phase()).toBe('ready');
    await nextTurn();
    expect(campaigns.getCampaign).not.toHaveBeenCalled();
    expect(api.get).toHaveBeenLastCalledWith('c1', 'm2');
  });

  it('asks for the role again while the person is still not the master', async () => {
    campaigns.getCampaign.mockResolvedValue(campaign(Role.PLAYER));
    page();
    await nextTurn();
    params.next(convertToParamMap({ id: 'c1', mapId: 'm2' }));
    await nextTurn();
    expect(campaigns.getCampaign).toHaveBeenCalledTimes(2);
  });

  it('ignores a route without both ids, and reads the "from" of the query', async () => {
    params.next(convertToParamMap({ id: 'c1' }));
    query.next(convertToParamMap({ from: 'session' }));
    const p = page();
    await nextTurn();
    expect(p.phase()).toBe('loading');
    expect(p.fromSession()).toBe(true);
    expect(campaigns.getCampaign).not.toHaveBeenCalled();
  });

  it('reads the session again when the window gets the focus (master) and the stream (player)', async () => {
    const master = page();
    await nextTurn();
    sessions.currentMap.mockClear();
    master.reloadFlags();
    await nextTurn();
    expect(sessions.currentMap).toHaveBeenCalledTimes(1);
    snapshotMapId = null;
    master.reloadFlags();
    await nextTurn();
    expect(sessions.currentMap).toHaveBeenCalledTimes(1);
  });

  it('asks a player page to open the stream again when the window gets the focus', async () => {
    campaigns.getCampaign.mockResolvedValue(campaign(Role.PLAYER));
    const p = page();
    await nextTurn();
    expect(watch).toHaveBeenCalledTimes(1);
    p.reloadFlags();
    await nextTurn();
    expect(sessions.currentMap).not.toHaveBeenCalled();
  });

  it('does nothing on the focus before the map is ready', () => {
    campaigns.getCampaign.mockReturnValue(new Promise(() => undefined));
    const p = page();
    p.reloadFlags();
    expect(sessions.currentMap).not.toHaveBeenCalled();
  });

  it('tries again on "tente de novo", and on nothing without a map in the route', async () => {
    const p = page();
    await nextTurn();
    api.get.mockClear();
    p.retry();
    await nextTurn();
    expect(api.get).toHaveBeenCalledTimes(1);
    api.get.mockClear();
    snapshotMapId = null;
    p.retry();
    await nextTurn();
    expect(api.get).not.toHaveBeenCalled();
  });

  it('keeps the list it had when the maps cannot be listed', async () => {
    const p = page();
    await nextTurn();
    api.list.mockRejectedValue(new Error('down'));
    await p.reloadMaps();
    expect(p.maps()).toHaveLength(1);
  });

  describe('the header calls', () => {
    async function ready(): Promise<PageView> {
      const p = page();
      await nextTurn();
      return p;
    }

    it('renames the map and reads the list again', async () => {
      const p = await ready();
      api.update.mockResolvedValue(mapMessage('m1', 'Nova', { revision: 2 }));
      api.list.mockClear();
      await p.saveName('Nova');
      expect(api.update).toHaveBeenCalledWith('c1', 'm1', 1, { name: 'Nova' });
      expect(p.state.map()?.name).toBe('Nova');
      expect(api.list).toHaveBeenCalled();
    });

    it('says why a rename failed', async () => {
      const p = await ready();
      api.update.mockRejectedValue(new ConnectError('x', Code.PermissionDenied));
      await expect(p.saveName('Nova')).rejects.toThrow('Só o mestre da campanha muda os mapas.');
    });

    it('does nothing to a map that is not open', async () => {
      campaigns.getCampaign.mockReturnValue(new Promise(() => undefined));
      const p = page();
      await p.saveName('x');
      await p.deleteMap();
      await p.toggleReveal();
      p.changeImage();
      expect(api.update).not.toHaveBeenCalled();
      expect(api.delete).not.toHaveBeenCalled();
      expect(api.setRevealed).not.toHaveBeenCalled();
      expect(dialog.open).not.toHaveBeenCalled();
    });

    it('deletes the map and goes back to the campaign', async () => {
      const p = await ready();
      await p.deleteMap();
      expect(api.delete).toHaveBeenCalledWith('c1', 'm1');
      expect(router.navigate).toHaveBeenCalledWith(['/campaigns', 'c1']);
    });

    it('says why a deletion failed, and stays', async () => {
      const p = await ready();
      api.delete.mockRejectedValue(new ConnectError('x', Code.Internal));
      await expect(p.deleteMap()).rejects.toThrow(/falar com o servidor/);
      expect(router.navigate).not.toHaveBeenCalled();
    });

    it('shows or hides the map, once at a time', async () => {
      const p = await ready();
      api.setRevealed.mockResolvedValue(mapMessage('m1', 'Cripta', { revealed: true }));
      const first = p.toggleReveal();
      expect(p.busy()).toBe(true);
      await p.toggleReveal();
      await first;
      expect(api.setRevealed).toHaveBeenCalledTimes(1);
      expect(api.setRevealed).toHaveBeenCalledWith('c1', 'm1', true);
      expect(p.state.map()?.revealed).toBe(true);
      expect(p.busy()).toBe(false);
    });

    it('shows the refusal of a toggle in the notice', async () => {
      const p = await ready();
      api.setRevealed.mockRejectedValue(new ConnectError('x', Code.Internal));
      await p.toggleReveal();
      expect(p.notice()).toContain('falar com o servidor');
      expect(p.busy()).toBe(false);
    });

    it('shows the generated image and reads the map again, and ignores an outcome without a map', async () => {
      const p = await ready();
      api.get.mockClear();
      p.imageGenerated({});
      expect(api.get).not.toHaveBeenCalled();
      p.imageGenerated({ map: create(MapSchema, { id: 'm1', name: 'Gerado' }) });
      expect(p.state.map()?.name).toBe('Gerado');
      expect(api.get).toHaveBeenCalled();
    });

    it('opens the image picker with the current image and saves the pick', async () => {
      const p = await ready();
      p.changeImage();
      expect(dialog.open).toHaveBeenCalledTimes(1);
      const data = dialog.open.mock.calls[0][1].data as {
        campaignId: string;
        current: { id: string; name: string } | null;
        note(): string;
        submit(image: { id: string }): Promise<void>;
        errorMessage(err: unknown): string;
      };
      expect(data.campaignId).toBe('c1');
      expect(data.current).toEqual({ id: 'img-m1', name: 'Imagem de Cripta' });
      expect(data.note()).toContain('posições são relativas');
      api.update.mockResolvedValue(mapMessage('m1', 'Cripta', { revision: 2 }));
      await data.submit({ id: 'i9' });
      expect(api.update).toHaveBeenCalledWith('c1', 'm1', 1, { imageId: 'i9' });
      sessions.currentMap.mockClear();
      expect(data.errorMessage(new ConnectError('x', Code.Internal))).toBeTruthy();
      await nextTurn();
      expect(sessions.currentMap).toHaveBeenCalled();
    });

    it('opens the image picker with no current image for a map that has none', async () => {
      const p = await ready();
      p.state.setMap(mapMessage('m1', 'Cripta', { image: undefined }));
      p.changeImage();
      expect(dialog.open.mock.calls[0][1].data.current).toBeNull();
    });
  });
});
