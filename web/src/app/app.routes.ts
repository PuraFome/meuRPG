import { Routes } from '@angular/router';

import { authGuard } from './core/auth/auth.guard';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () => import('./pages/home/home').then((m) => m.Home),
  },
  {
    path: 'campanhas',
    canActivate: [authGuard],
    loadComponent: () => import('./pages/campaigns/campaigns').then((m) => m.Campaigns),
  },
  {
    // Where authGuard sends a guarded navigation when the session state is
    // `unavailable` (see the guard's doc comment). Not in the toolbar nav.
    path: 'indisponivel',
    loadComponent: () =>
      import('./pages/server-unavailable/server-unavailable').then((m) => m.ServerUnavailable),
  },
  {
    path: '**',
    loadComponent: () => import('./pages/not-found/not-found').then((m) => m.NotFound),
  },
];
