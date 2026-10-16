import { signal } from '@angular/core';
import { convertToParamMap } from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';

import type { AbilityTableVm, CharacterForEdit, RulesCatalogVm } from './character-editor.types';
import { EditorLoader, type EditorLoaderContext } from './editor-loader';
import type { PageState, ReadyState } from './editor-state';
import { nextTurn } from '../../../test-settle';

const lists = (marker = 'a'): RulesCatalogVm =>
  ({ races: [], classes: [], backgrounds: [], spells: [], viewerIsMaster: false, marker }) as never;

const existing = (over: Partial<CharacterForEdit> = {}): CharacterForEdit => ({
  kind: 'player',
  revision: 2,
  blocked: null,
  sheetLocked: false,
  full: null,
  basic: null,
  ...over,
});

const TABLE = { hitPoints: 'player_chooses' } as AbilityTableVm;

describe('EditorLoader', () => {
  let state = signal<PageState>({ status: 'loading', title: 'Ficha' });
  let abilityTable = signal<AbilityTableVm | null>(null);
  let lockedOrigin = signal<ReturnType<EditorLoaderContext['lockedOrigin']>>(null);
  let contentNote = signal('');
  let campaignId = signal('');
  let resets = 0;
  let createdWith: { ready: ReadyState; table: AbilityTableVm | null }[] = [];
  let editedWith: { ready: ReadyState; existing: CharacterForEdit }[] = [];
  let reserved = false;
  let catalogFn: (campaign: string, character?: string) => Promise<RulesCatalogVm>;
  let editFn: () => Promise<CharacterForEdit>;
  let tableFn: () => Promise<AbilityTableVm | null>;
  let tableCalls = 0;

  function loader(): EditorLoader {
    return new EditorLoader({
      source: {
        loadCatalog: (c, ch) => catalogFn(c, ch),
        loadCharacterForEdit: () => editFn(),
        loadAbilityTable: () => {
          tableCalls++;
          return tableFn();
        },
      },
      state,
      abilityTable,
      lockedOrigin,
      contentNote,
      campaignId,
      reservedRoute: () => reserved,
      reset: () => resets++,
      created: (ready, table) => {
        createdWith.push({ ready, table });
        state.set(ready);
      },
      edited: (ready, e) => {
        editedWith.push({ ready, existing: e });
        state.set(ready);
      },
    });
  }

  beforeEach(() => {
    state = signal<PageState>({ status: 'loading', title: 'Ficha' });
    abilityTable = signal<AbilityTableVm | null>(null);
    lockedOrigin = signal(null);
    contentNote = signal('');
    campaignId = signal('');
    resets = 0;
    createdWith = [];
    editedWith = [];
    reserved = false;
    tableCalls = 0;
    catalogFn = () => Promise.resolve(lists());
    editFn = () => Promise.resolve(existing());
    tableFn = () => Promise.resolve(TABLE);
  });

  describe('the route', () => {
    it('loads nothing without a campaign', () => {
      loader().resolve(convertToParamMap({}));
      expect(resets).toBe(0);
      expect(campaignId()).toBe('');
    });

    it('opens an edit for a route with a character', async () => {
      loader().resolve(convertToParamMap({ id: 'camp', characterId: 'char' }));
      await nextTurn();
      expect(campaignId()).toBe('camp');
      expect(editedWith).toHaveLength(1);
      expect(editedWith[0].ready).toMatchObject({ mode: 'edit', characterId: 'char' });
    });

    it('opens a new player, or the NPC kind the route names (an enemy for a kind it does not know)', async () => {
      const load = loader();
      load.resolve(convertToParamMap({ id: 'camp' }));
      await nextTurn();
      load.resolve(convertToParamMap({ id: 'camp', kind: 'boss' }));
      await nextTurn();
      load.resolve(convertToParamMap({ id: 'camp', kind: 'dragon' }));
      await nextTurn();
      expect(createdWith.map((c) => c.ready.kind)).toEqual(['player', 'boss', 'enemy']);
    });

    it('opens a reserved character where the route says so', async () => {
      reserved = true;
      loader().resolve(convertToParamMap({ id: 'camp' }));
      await nextTurn();
      expect(createdWith[0].ready.reserved).toBe(true);
      expect(createdWith[0].table).toBeNull();
      expect(tableCalls).toBe(0);
    });
  });

  describe('a new sheet', () => {
    it('resets the page, shows it loading, then hands over the lists and the table of a player', async () => {
      const load = loader();
      load.forCreate('camp', 'player', false);
      expect(resets).toBe(1);
      expect(state()).toEqual({ status: 'loading', title: 'Criar personagem' });
      await nextTurn();
      expect(createdWith[0].table).toBe(TABLE);
      expect(createdWith[0].ready).toMatchObject({ mode: 'create', revision: 1, kind: 'player' });
    });

    it('asks no table for an NPC', async () => {
      loader().forCreate('camp', 'enemy', false);
      await nextTurn();
      expect(tableCalls).toBe(0);
      expect(createdWith[0].table).toBeNull();
    });

    it('says the lists could not be read, with a way back', async () => {
      catalogFn = () => Promise.reject(new ConnectError('x', Code.PermissionDenied));
      loader().forCreate('camp', 'player', false);
      await nextTurn();
      expect(state()).toMatchObject({ status: 'error', backLink: ['/campaigns', 'camp'] });
    });

    it('drops the answer of a route the person left, good or bad', async () => {
      const answers: { ok: (c: RulesCatalogVm) => void; fail: (e: unknown) => void }[] = [];
      catalogFn = () => new Promise((ok, fail) => answers.push({ ok, fail }));
      const load = loader();
      load.forCreate('camp', 'player', false);
      load.forCreate('camp-2', 'player', false);
      load.forCreate('camp-3', 'player', false);
      answers[0].ok(lists());
      answers[1].fail(new Error('late'));
      await nextTurn();
      expect(createdWith).toHaveLength(0);
      expect(state().status).toBe('loading');
    });
  });

  describe('an existing sheet', () => {
    it('reads the lists with the sheet, so what it has comes back even when retired', async () => {
      const seen: (string | undefined)[] = [];
      catalogFn = (_c, ch) => {
        seen.push(ch);
        return Promise.resolve(lists());
      };
      loader().forEdit('camp', 'char');
      await nextTurn();
      expect(seen).toEqual(['char']);
    });

    it("keeps the way a player's scores were made, with the table", async () => {
      const origin = { method: 'point_buy' as const, rolls: null };
      editFn = () => Promise.resolve(existing({ abilityOrigin: origin }));
      loader().forEdit('camp', 'char');
      await nextTurn();
      expect(abilityTable()).toBe(TABLE);
      expect(lockedOrigin()).toBe(origin);
    });

    it('keeps the free editor for an NPC, or a sheet made before the rules', async () => {
      editFn = () =>
        Promise.resolve(
          existing({ kind: 'enemy', abilityOrigin: { method: 'typed', rolls: null } }),
        );
      loader().forEdit('camp', 'char');
      await nextTurn();
      expect(tableCalls).toBe(0);
      expect(abilityTable()).toBeNull();
      expect(lockedOrigin()).toBeNull();
    });

    it('says a blocked sheet is blocked instead of opening the form', async () => {
      editFn = () => Promise.resolve(existing({ blocked: 'sheet_locked' }));
      loader().forEdit('camp', 'char');
      await nextTurn();
      expect(editedWith).toEqual([]);
      expect(state()).toMatchObject({ status: 'error', blocked: { title: 'Ficha travada' } });
    });

    it('says the sheet could not be read, with a way back to it', async () => {
      editFn = () => Promise.reject(new ConnectError('x', Code.NotFound));
      loader().forEdit('camp', 'char');
      await nextTurn();
      expect(state()).toMatchObject({
        status: 'error',
        message: 'Personagem não encontrado.',
        backLink: ['/campaigns', 'camp', 'characters', 'char'],
      });
    });

    it('drops a late answer, a late table and a late failure of a route the person left', async () => {
      let answer: (e: CharacterForEdit) => void = () => undefined;
      editFn = () => new Promise((ok) => (answer = ok));
      const load = loader();
      load.forEdit('camp', 'char');
      load.destroy();
      answer(existing({ abilityOrigin: { method: 'typed', rolls: null } }));
      await nextTurn();
      expect(editedWith).toEqual([]);
      expect(abilityTable()).toBeNull();

      let fail: (e: unknown) => void = () => undefined;
      editFn = () => new Promise((_, reject) => (fail = reject));
      const other = loader();
      other.forEdit('camp', 'char');
      other.seq++;
      fail(new Error('late'));
      await nextTurn();
      expect(state().status).toBe('loading');
    });
  });

  describe('the table of ways to make scores', () => {
    const readyState = (): PageState => ({
      status: 'ready',
      mode: 'create',
      kind: 'player',
      campaignId: 'camp',
      characterId: null,
      revision: 1,
      catalog: lists(),
      xpLocked: false,
      reserved: false,
    });

    it('is read again only for a sheet that has one', async () => {
      const load = loader();
      await load.rereadAbilityTable();
      state.set(readyState());
      await load.rereadAbilityTable();
      expect(tableCalls).toBe(0);
      abilityTable.set(TABLE);
      const fresh = { ...TABLE, hitPoints: 'roll' } as AbilityTableVm;
      tableFn = () => Promise.resolve(fresh);
      await load.rereadAbilityTable();
      expect(abilityTable()).toBe(fresh);
    });

    it('keeps the one it has when the server has none, fails, or the route changed', async () => {
      const load = loader();
      state.set(readyState());
      abilityTable.set(TABLE);
      tableFn = () => Promise.resolve(null);
      await load.rereadAbilityTable();
      tableFn = () => Promise.reject(new Error('x'));
      await load.rereadAbilityTable();
      let answer: (t: AbilityTableVm | null) => void = () => undefined;
      tableFn = () => new Promise((ok) => (answer = ok));
      const pending = load.rereadAbilityTable();
      load.seq++;
      answer({ ...TABLE, hitPoints: 'roll' } as AbilityTableVm);
      await pending;
      expect(abilityTable()).toBe(TABLE);
    });
  });

  describe('the lists after a content change', () => {
    const readyWith = (catalog: RulesCatalogVm): PageState => ({
      status: 'ready',
      mode: 'edit',
      kind: 'player',
      campaignId: 'camp',
      characterId: 'char',
      revision: 1,
      catalog,
      xpLocked: false,
      reserved: false,
    });
    const withRace = (off: boolean): RulesCatalogVm =>
      ({
        ...lists(),
        races: [
          {
            key: 'race:gnome',
            namePt: 'Gnomo',
            constitutionBonus: 0,
            choiceBonuses: [],
            subraces: [],
            fromTable: true,
            archived: false,
            off,
          },
        ],
      }) as never;

    it('does nothing before the page is open', async () => {
      await loader().refreshCatalog();
      expect(state().status).toBe('loading');
    });

    it('swaps the lists and says so when what the person is offered changed', async () => {
      state.set(readyWith(withRace(false)));
      catalogFn = () => Promise.resolve(withRace(true));
      await loader().refreshCatalog();
      expect((state() as ReadyState).catalog.races[0].off).toBe(true);
      expect(contentNote()).toContain('O mestre mudou as opções da mesa');
    });

    it("swaps the lists without a notice when only the table's own entries changed", async () => {
      const before = withRace(false);
      state.set(readyWith(before));
      const after = {
        ...before,
        races: [{ ...before.races[0], constitutionBonus: 2 }],
      } as RulesCatalogVm;
      catalogFn = () => Promise.resolve(after);
      await loader().refreshCatalog();
      expect((state() as ReadyState).catalog).toBe(after);
      expect(contentNote()).toBe('');
    });

    it('keeps the lists when nothing changed, when the read fails, or when a newer read or route took over', async () => {
      const before = withRace(false);
      state.set(readyWith(before));
      const load = loader();
      catalogFn = () => Promise.resolve(withRace(false));
      await load.refreshCatalog();
      expect((state() as ReadyState).catalog).toBe(before);
      catalogFn = () => Promise.reject(new Error('x'));
      await load.refreshCatalog();
      expect((state() as ReadyState).catalog).toBe(before);

      const answers: ((c: RulesCatalogVm) => void)[] = [];
      catalogFn = () => new Promise((ok) => answers.push(ok));
      const older = load.refreshCatalog();
      const newer = load.refreshCatalog();
      answers[0](withRace(true));
      answers[1](before);
      await Promise.all([older, newer]);
      expect((state() as ReadyState).catalog).toBe(before);

      const routed = load.refreshCatalog();
      load.seq++;
      answers[2](withRace(true));
      await routed;
      expect((state() as ReadyState).catalog).toBe(before);
    });
  });

  describe('a save that outlives its route', () => {
    it('knows whether the person is still on the page and route it began on', () => {
      const load = loader();
      const stillHere = load.stillHere();
      expect(stillHere()).toBe(true);
      load.seq++;
      expect(stillHere()).toBe(false);
      const second = load.stillHere();
      load.destroy();
      expect(second()).toBe(false);
      expect(load.stillHere()()).toBe(false);
    });
  });
});
