import { Routes } from '@angular/router';

import { CharacterEditorSource } from './character-editor.types';
import { CharacterEditorSourceLive } from './character-editor-source.live';

/**
 * Lazily loaded from `app.routes.ts` via `loadChildren`, from all three
 * places the editor is reachable (`/campaigns/:id/characters/new`,
 * `/campaigns/:id/npcs/new/:kind`, `/campaigns/:id/characters/:characterId/edit`)
 * — see `character-sheet.routes.ts`'s doc comment for why `loadChildren`
 * instead of a route-level `providers` array directly in `app.routes.ts`.
 */
export const CHARACTER_EDITOR_ROUTES: Routes = [
  {
    path: '',
    providers: [
      { provide: CharacterEditorSource, useClass: CharacterEditorSourceLive },
    ],
    loadComponent: () => import('./character-editor').then((m) => m.CharacterEditor),
  },
];
