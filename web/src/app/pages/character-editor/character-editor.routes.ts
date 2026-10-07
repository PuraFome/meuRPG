import { Routes } from '@angular/router';

import { CharacterEditorSource } from './character-editor.types';
import { CharacterEditorSourceLive } from './character-editor-source.live';
import { LiveSessionSourceLive } from '../live-session/live-session-source.live';
import { XpWatcher } from '../character-sheet/xp-watcher';

/**
 * Lazily loaded from `app.routes.ts` via `loadChildren`, from all three
 * places the editor is reachable (`/campanhas/:id/personagens/novo`,
 * `/campanhas/:id/npcs/novo/:tipo`, `/campanhas/:id/personagens/:characterId/editar`)
 * — see `character-sheet.routes.ts`'s doc comment for why `loadChildren`
 * instead of a route-level `providers` array directly in `app.routes.ts`.
 */
export const CHARACTER_EDITOR_ROUTES: Routes = [
  {
    path: '',
    providers: [
      { provide: CharacterEditorSource, useClass: CharacterEditorSourceLive },
      // The session's stream, for `content_changed` (10.1d): the catalog is read again.
      LiveSessionSourceLive,
      XpWatcher,
    ],
    loadComponent: () => import('./character-editor').then((m) => m.CharacterEditor),
  },
];
