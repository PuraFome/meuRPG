import { Routes } from '@angular/router';

import { LiveSessionSourceLive } from './live-session-source.live';
import { LiveSessionSource } from './live-session.types';

/**
 * Lazily loaded from `app.routes.ts` via `loadChildren` for
 * `/campanhas/:id/sessao`, so `LiveSessionSource` gets a route-scoped
 * provider without `app.routes.ts` (eager) importing the generated clients
 * behind it — see `../character-sheet/character-sheet.routes.ts`.
 */
export const LIVE_SESSION_ROUTES: Routes = [
  {
    path: '',
    providers: [{ provide: LiveSessionSource, useClass: LiveSessionSourceLive }],
    loadComponent: () => import('./live-session').then((m) => m.LiveSession),
  },
];
