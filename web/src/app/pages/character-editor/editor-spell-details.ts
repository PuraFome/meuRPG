import type { Signal } from '@angular/core';
import type { MatBottomSheet } from '@angular/material/bottom-sheet';
import type { MatDialog } from '@angular/material/dialog';

import { openSpellDetails } from '../../shared/spell-details/open-spell-details';
import type { SpellDetailsVm } from '../../shared/spell-details/spell-details.types';
import type { CharacterEditorSource, SpellOptionVm } from './character-editor.types';
import type { PageState } from './editor-state';

/** What `EditorSpellDetails` reads from the page. */
export interface EditorSpellDetailsHost {
  readonly source: Pick<CharacterEditorSource, 'loadSpellDetails'>;
  readonly state: Signal<PageState>;
  readonly dialog: MatDialog;
  readonly bottomSheet: MatBottomSheet;
}

/**
 * The "?" next to a spell: its description, as a bottom sheet on a phone and a dialog from a tablet up. The
 * descriptions already fetched are kept by spell key for the life of the page (a second "?" on the same spell is
 * instant; nothing is stored in the browser). A failed fetch is dropped so "Tentar de novo" asks again.
 */
export class EditorSpellDetails {
  private readonly cache = new Map<string, Promise<SpellDetailsVm>>();

  constructor(private readonly host: EditorSpellDetailsHost) {}

  /** Focus goes back to the "?" on close. */
  describe(spell: SpellOptionVm): void {
    const s = this.host.state();
    if (s.status !== 'ready') {
      return;
    }
    openSpellDetails(this.host.dialog, this.host.bottomSheet, {
      namePt: spell.namePt,
      load: () => this.load(s.campaignId, spell.key),
    });
  }

  private load(campaignId: string, key: string): Promise<SpellDetailsVm> {
    let pending = this.cache.get(key);
    if (!pending) {
      pending = this.host.source.loadSpellDetails(campaignId, key);
      this.cache.set(key, pending);
      pending.catch(() => this.cache.delete(key));
    }
    return pending;
  }
}
