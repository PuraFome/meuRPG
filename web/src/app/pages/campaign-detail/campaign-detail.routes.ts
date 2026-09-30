import { Routes } from '@angular/router';

import { CampaignCharactersSource } from './characters/campaign-characters.types';
import { CampaignCharactersSourceLive } from './characters/campaign-characters-source.live';
import { GameSessionSource } from './game-session/game-session-card.types';
import { GameSessionSourceLive } from './game-session/game-session-source.live';

/**
 * Lazily loaded from `app.routes.ts` via `loadChildren` for `/campanhas/:id`
 * — see `../character-sheet/character-sheet.routes.ts`'s doc comment for
 * why. `CampaignDetail` renders `<app-campaign-characters>` and (for the
 * master) `<app-game-session-card>` directly in its template, and both
 * resolve their source through this route's environment injector, same as
 * `CampaignDetail` itself.
 */
export const CAMPAIGN_DETAIL_ROUTES: Routes = [
  {
    path: '',
    providers: [
      { provide: CampaignCharactersSource, useClass: CampaignCharactersSourceLive },
      { provide: GameSessionSource, useClass: GameSessionSourceLive },
    ],
    loadComponent: () => import('./campaign-detail').then((m) => m.CampaignDetail),
  },
];
