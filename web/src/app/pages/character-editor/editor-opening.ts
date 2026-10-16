import type { WritableSignal } from '@angular/core';
import type { FormBuilder } from '@angular/forms';

import type {
  AbilityMethodKey,
  AbilityTableVm,
  CharacterForEdit,
  CharacterFormValue,
  RulesCatalogVm,
} from './character-editor.types';
import type { EditorCatalog } from './editor-catalog';
import type { EditorCore } from './editor-core';
import { type ReadyState, tableHitPointsMethod } from './editor-state';
import { fullFormValueOf } from './full-form';
import { patchBasicForm } from './npc-short-form/basic-form';

/** What `EditorOpening` reads from, and fills in on, the page. */
export interface EditorOpeningHost extends EditorCore {
  readonly fb: FormBuilder;
  readonly views: EditorCatalog;
  readonly abilityTable: WritableSignal<AbilityTableVm | null>;
  readonly abilityMethod: WritableSignal<AbilityMethodKey>;
  readonly lockedOrigin: WritableSignal<NonNullable<CharacterForEdit['abilityOrigin']> | null>;
  readonly contentNote: WritableSignal<string>;
  /** The sheet's own state that is not a signal here: the save and the server reads of the draft start over. */
  readonly forget: () => void;
}

/**
 * What the page does when `EditorLoader` has the lists: starts from nothing when the component is reused for another
 * route, fills the defaults of a new sheet, and opens the form with what a saved sheet has. A plain class the editor
 * builds; the signals it fills are the page's.
 */
export class EditorOpening {
  constructor(private readonly host: EditorOpeningHost) {}

  /** A reused component starts every route from nothing: nothing of the previous character stays in the form or the pick sets. */
  reset(): void {
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

  /** The lists of a new sheet arrived (`EditorLoader`). */
  created(ready: ReadyState, table: AbilityTableVm | null): void {
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

  /** The lists and the sheet of an edit arrived (`EditorLoader`). */
  edited(ready: ReadyState, existing: CharacterForEdit, catalog: RulesCatalogVm): void {
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
}
