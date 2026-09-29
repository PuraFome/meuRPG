import { Routes } from '@angular/router';

import { CharacterSheetSource } from './character-sheet.types';
import { CharacterSheetSourceLive } from './character-sheet-source.live';

/**
 * Lazily loaded from `app.routes.ts` via `loadChildren`, instead of a plain
 * `loadComponent` — the only way to give `CharacterSheetSource` a real,
 * route-scoped provider without importing `CharacterSheetSourceLive` (and
 * the generated `characters_pb.ts` / `rules_pb.ts` it pulls in) from
 * `app.routes.ts` itself, which is part of the eager bundle. `authGuard`
 * still runs first, from the parent route in `app.routes.ts`.
 */
export const CHARACTER_SHEET_ROUTES: Routes = [
  {
    path: '',
    providers: [{ provide: CharacterSheetSource, useClass: CharacterSheetSourceLive }],
    loadComponent: () => import('./character-sheet').then((m) => m.CharacterSheetPage),
  },
];
