import type { WritableSignal } from '@angular/core';

import type { EditorPicks } from './editor-picks';
import type { PageState } from './editor-state';
import type { FullForm } from './full-form';
import type { BasicSheetFormGroup } from './npc-short-form/basic-form';
import type { SpellPicks } from './spell-picks';

/** What every plain class the editor builds reads: the page's state, its two forms and its picks. */
export interface EditorCore {
  readonly state: WritableSignal<PageState>;
  readonly fullForm: FullForm;
  readonly basicForm: BasicSheetFormGroup;
  readonly picks: EditorPicks;
  readonly spellPicks: SpellPicks;
}
