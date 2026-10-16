import type { WritableSignal } from '@angular/core';
import type { ParamMap } from '@angular/router';

import { CharacterKind } from '../../core/characters/characters.types';
import { catalogChanged, offersChanged } from './catalog-changes';
import type {
  AbilityTableVm,
  CharacterEditorSource,
  CharacterForEdit,
  RulesCatalogVm,
} from './character-editor.types';
import {
  NPC_ROUTE_KINDS,
  type PageState,
  type ReadyState,
  createFailed,
  editBlocked,
  editFailed,
  loadingState,
  readyForCreate,
  readyForEdit,
} from './editor-state';

/** What `EditorLoader` reads from, and hands back to, the page it loads for. */
export interface EditorLoaderContext {
  readonly source: Pick<
    CharacterEditorSource,
    'loadCatalog' | 'loadCharacterForEdit' | 'loadAbilityTable'
  >;
  readonly state: WritableSignal<PageState>;
  readonly abilityTable: WritableSignal<AbilityTableVm | null>;
  readonly lockedOrigin: WritableSignal<NonNullable<CharacterForEdit['abilityOrigin']> | null>;
  readonly contentNote: WritableSignal<string>;
  readonly campaignId: WritableSignal<string>;
  /** The route is `/campaigns/:id/reserved/new`: the master makes a character for a player to claim. */
  readonly reservedRoute: () => boolean;
  /** A reused component starts every route from nothing: nothing of the previous character stays. */
  readonly reset: () => void;
  /** The lists of a new sheet arrived: the page takes the table, fills the defaults and opens the form. */
  readonly created: (ready: ReadyState, table: AbilityTableVm | null) => void;
  /** The lists and the sheet of an edit arrived: the page opens the form with what the sheet has. */
  readonly edited: (ready: ReadyState, existing: CharacterForEdit, catalog: RulesCatalogVm) => void;
}

/**
 * What the editor reads from the server to open, and how it drops the answers of a route the person left: the lists
 * of a new sheet, the lists and the sheet of an edit, the table of ways to make scores, and the lists again after a
 * `content_changed`. Plain class the editor builds; it counts the loads, so a late answer for a route the person left
 * is dropped, and a save that outlives its route does not navigate.
 */
export class EditorLoader {
  /** Counts the loads, so a late answer for a route the person left is dropped. */
  seq = 0;
  /** Counts the reads of the lists after a `content_changed`, so an older answer never replaces a newer one. */
  private catalogSeq = 0;
  private destroyed = false;

  constructor(private readonly host: EditorLoaderContext) {}

  /** The page is gone: no answer lands, and a save does not navigate. */
  destroy(): void {
    this.destroyed = true;
    this.seq++;
  }

  /** Taken when a save starts: says whether the person is still on the page and route it began on. */
  stillHere(): () => boolean {
    const seq = this.seq;
    return () => !this.destroyed && seq === this.seq;
  }

  /** The route changed: loads for the sheet it names. */
  resolve(params: ParamMap): void {
    const campaignId = params.get('id');
    if (!campaignId) {
      return;
    }
    const characterId = params.get('characterId');
    const npcKind = params.get('kind');
    this.host.campaignId.set(campaignId);

    if (characterId) {
      this.forEdit(campaignId, characterId);
      return;
    }
    const kind: CharacterKind = npcKind ? (NPC_ROUTE_KINDS[npcKind] ?? 'enemy') : 'player';
    // `/campaigns/:id/reserved/new` (MR-049): the master makes a player character for a player to claim.
    this.forCreate(campaignId, kind, this.host.reservedRoute());
  }

  forCreate(campaignId: string, kind: CharacterKind, reserved: boolean): void {
    const seq = ++this.seq;
    this.host.reset();
    this.host.state.set(loadingState('create', kind, reserved));
    // A player (or a pending member) makes the scores the table's rules allow; the master's NPCs, and the characters
    // the master makes for a player to claim, are free.
    const table =
      kind === 'player' && !reserved
        ? this.host.source.loadAbilityTable(campaignId)
        : Promise.resolve(null);
    Promise.all([this.host.source.loadCatalog(campaignId), table]).then(
      ([catalog, abilityTable]) => {
        if (seq !== this.seq) {
          return;
        }
        this.host.created(readyForCreate(kind, campaignId, catalog, reserved), abilityTable);
      },
      (err: unknown) => {
        if (seq !== this.seq) {
          return;
        }
        this.host.state.set(createFailed(err, campaignId));
      },
    );
  }

  forEdit(campaignId: string, characterId: string): void {
    const seq = ++this.seq;
    this.host.reset();
    this.host.state.set(loadingState('edit', 'player'));
    Promise.all([
      // With the sheet: what it has comes back even when the master retired it since.
      this.host.source.loadCatalog(campaignId, characterId),
      this.host.source.loadCharacterForEdit(campaignId, characterId),
    ])
      .then(async ([catalog, existing]) => {
        // A player's own draft keeps the way its scores were made (RN-24): the step shows that way and its limits.
        // The master (and an NPC, or a sheet made before the rules) keeps the free editor.
        const origin = existing.abilityOrigin ?? null;
        const table =
          origin && existing.kind === 'player'
            ? await this.host.source.loadAbilityTable(campaignId)
            : null;
        if (seq !== this.seq) {
          return null;
        }
        this.host.abilityTable.set(table);
        this.host.lockedOrigin.set(table ? origin : null);
        return [catalog, existing] as const;
      })
      .then((loaded) => {
        if (loaded === null || seq !== this.seq) {
          return;
        }
        const [catalog, existing] = loaded;
        if (existing.blocked) {
          this.host.state.set(editBlocked(existing.blocked, campaignId, characterId));
          return;
        }
        this.host.edited(
          readyForEdit(existing, campaignId, characterId, catalog),
          existing,
          catalog,
        );
      })
      .catch((err: unknown) => {
        if (seq !== this.seq) {
          return;
        }
        this.host.state.set(editFailed(err, campaignId, characterId));
      });
  }

  /** The table's rules for the scores changed under the person (the server refused the way of rolling the step
   * offered): they are read again, so the step offers what is allowed now. */
  async rereadAbilityTable(): Promise<void> {
    const s = this.host.state();
    if (s.status !== 'ready' || !this.host.abilityTable()) {
      return;
    }
    const seq = this.seq;
    try {
      const table = await this.host.source.loadAbilityTable(s.campaignId);
      if (seq === this.seq && table) {
        this.host.abilityTable.set(table);
      }
    } catch {
      // The refusal the step already shows stays; the next try reads the table again.
    }
  }

  /** `content_changed` (RN-23): the lists are read again with this person's role; what was typed stays. Resolves when done. */
  async refreshCatalog(): Promise<void> {
    const s = this.host.state();
    if (s.status !== 'ready') {
      return;
    }
    // The newest read is the one that counts, and an answer for a route the person left does not count at all.
    const seq = ++this.catalogSeq;
    const route = this.seq;
    try {
      const catalog = await this.host.source.loadCatalog(s.campaignId, s.characterId ?? undefined);
      if (seq !== this.catalogSeq || route !== this.seq) {
        return;
      }
      const now = this.host.state();
      if (now.status === 'ready' && catalogChanged(now.catalog, catalog)) {
        this.host.state.set({ ...now, catalog });
        if (offersChanged(now.catalog, catalog)) {
          this.host.contentNote.set(
            'O mestre mudou as opções da mesa. As listas foram atualizadas: confira o que você escolheu antes de salvar.',
          );
        }
      }
    } catch {
      // Keep the lists on screen: the next change reads again.
    }
  }
}
