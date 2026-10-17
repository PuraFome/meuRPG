import type { WritableSignal } from '@angular/core';
import type { FormBuilder } from '@angular/forms';
import type { ParamMap } from '@angular/router';

import { CharacterKind } from '../../core/characters/characters.types';
import { catalogChanged, offersChanged } from './catalog-changes';
import type {
  AbilityMethodKey,
  AbilityTableVm,
  CharacterEditorSource,
  CharacterForEdit,
  CharacterFormValue,
  RulesCatalogVm,
} from './character-editor.types';
import type { EditorCatalog } from './editor-catalog';
import type { EditorCore } from './editor-core';
import {
  NPC_ROUTE_KINDS,
  type ReadyState,
  createFailed,
  editBlocked,
  editFailed,
  loadingState,
  readyForCreate,
  readyForEdit,
  tableHitPointsMethod,
} from './editor-state';
import { fullFormValueOf } from './full-form';
import { patchBasicForm } from './npc-short-form/basic-form';

/** What `EditorLoader` reads from, and fills in on, the page it loads for. */
export interface EditorLoaderContext extends EditorCore {
  readonly source: Pick<
    CharacterEditorSource,
    'loadCatalog' | 'loadCharacterForEdit' | 'loadAbilityTable'
  >;
  readonly fb: FormBuilder;
  readonly views: Pick<EditorCatalog, 'sections'>;
  readonly abilityTable: WritableSignal<AbilityTableVm | null>;
  readonly abilityMethod: WritableSignal<AbilityMethodKey>;
  readonly lockedOrigin: WritableSignal<NonNullable<CharacterForEdit['abilityOrigin']> | null>;
  readonly contentNote: WritableSignal<string>;
  readonly campaignId: WritableSignal<string>;
  /** The route is `/campaigns/:id/reserved/new`: the master makes a character for a player to claim. */
  readonly reservedRoute: () => boolean;
  /** The sheet's own state that is not a signal here: the save and the server reads of the draft start over. */
  readonly forget: () => void;
}

/**
 * What the editor reads from the server to open, and what it does with the answer: the lists of a new sheet (the
 * defaults of the form), the lists and the sheet of an edit (the form and the picks of what the sheet has), the table
 * of ways to make scores, and the lists again after a `content_changed`. Plain class the editor builds; it counts the
 * loads, so a late answer for a route the person left is dropped, and a save that outlives its route does not
 * navigate.
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
    this.reset();
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
        this.created(readyForCreate(kind, campaignId, catalog, reserved), abilityTable);
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
    this.reset();
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
        this.edited(readyForEdit(existing, campaignId, characterId, catalog), existing, catalog);
      })
      .catch((err: unknown) => {
        if (seq !== this.seq) {
          return;
        }
        this.host.state.set(editFailed(err, campaignId, characterId));
      });
  }

  /** A reused component starts every route from nothing: nothing of the previous character stays in the form or the pick sets. */
  private reset(): void {
    const h = this.host;
    h.fullForm.reset();
    h.basicForm.reset();
    h.picks.reset();
    h.spellPicks.reset();
    h.abilityTable.set(null);
    h.abilityMethod.set('typed');
    h.lockedOrigin.set(null);
    h.contentNote.set('');
    h.forget();
  }

  /** The lists of a new sheet arrived: the defaults of the form. */
  private created(ready: ReadyState, table: AbilityTableVm | null): void {
    const h = this.host;
    h.abilityTable.set(table);
    const method = tableHitPointsMethod(table);
    if (method) {
      h.fullForm.controls.hitPointsMethod.setValue(method);
    }
    h.state.set(ready);
    // A new enemy, boss or minion starts at ND 0 and 10 XP, so nobody is left without a
    // number; a story NPC gives none (and never shows the fields).
    if (ready.kind === 'enemy' || ready.kind === 'boss') {
      h.fullForm.patchValue({ challengeRating: '0', xpValue: 10 });
    } else if (ready.kind === 'story') {
      h.basicForm.patchValue({ challengeRating: '', xpValue: 0 });
    }
  }

  /** The lists and the sheet of an edit arrived: the form opens with what the sheet has. */
  private edited(ready: ReadyState, existing: CharacterForEdit, catalog: RulesCatalogVm): void {
    this.host.state.set(ready);
    if (existing.full) {
      this.openSavedSheet(existing, existing.full, catalog);
    }
    if (existing.basic) {
      patchBasicForm(this.host.fb, this.host.basicForm, existing.basic);
    }
  }

  private openSavedSheet(
    existing: CharacterForEdit,
    full: CharacterFormValue,
    catalog: RulesCatalogVm,
  ): void {
    const { fullForm, picks, spellPicks, views } = this.host;
    fullForm.setValue(fullFormValueOf(full));
    picks.load(full);
    spellPicks.load(full.cantrips, full.spellsKnown, full.spellsPrepared);
    picks.preparedMaxByClass.set(existing.preparedMax ?? {});
    picks.hasCuttingWords.set((existing.featureKeys ?? []).includes('feature:cutting-words'));
    // What the sheet has from outside the form is told once, as the sheet has it: a spell of the subclass it
    // has now stays out of this list, and stays out when the subclass is changed (the save drops it with the subclass).
    const granted = new Set(existing.grantedSpellKeys ?? []);
    const fromSubclass = new Set(views.sections().flatMap((sec) => sec.alwaysPrepared));
    picks.grantedSpells.set(
      catalog.spells.filter((sp) => granted.has(sp.key) && !fromSubclass.has(sp.key)),
    );
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
