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
    path: 'campanhas/:id',
    canActivate: [authGuard],
    loadComponent: () =>
      import('./pages/campaign-detail/campaign-detail').then((m) => m.CampaignDetail),
  },
  {
    // Public: read from the invite link's fragment (never a route param —
    // see InviteAccept's doc comment) and works whether the visitor is
    // signed in or not (MR-003).
    path: 'convite',
    loadComponent: () => import('./pages/invite/invite-accept').then((m) => m.InviteAccept),
  },
  {
    // Where the server redirects after sign-in-through-invite when the
    // invite could not be accepted (docs/arquitetura.md#frontend-web).
    path: 'convite/erro',
    loadComponent: () => import('./pages/invite-error/invite-error').then((m) => m.InviteError),
  },
  {
    path: 'perfil',
    canActivate: [authGuard],
    loadComponent: () => import('./pages/profile/profile').then((m) => m.Profile),
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
