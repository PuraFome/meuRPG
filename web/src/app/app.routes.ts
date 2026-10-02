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
    // `loadChildren`, not `loadComponent`: CampaignDetail's route needs its
    // own `providers` (CampaignCharactersSource, GameSessionSource, phase
    // 2) without pulling their generated Connect clients into this file,
    // which is part of the eager bundle — see campaign-detail.routes.ts's
    // doc comment.
    path: 'campanhas/:id',
    canActivate: [authGuard],
    loadChildren: () =>
      import('./pages/campaign-detail/campaign-detail.routes').then((m) => m.CAMPAIGN_DETAIL_ROUTES),
  },
  {
    // The live session (MR-011, MR-012, RN-06, RN-07): the link the master
    // copies. No secret in it: the server decides who gets in. Signed out,
    // authGuard sends the person to sign in and back here. `loadChildren`
    // for the same reason as `campanhas/:id` above — see
    // live-session.routes.ts.
    path: 'campanhas/:id/sessao',
    canActivate: [authGuard],
    loadChildren: () =>
      import('./pages/live-session/live-session.routes').then((m) => m.LIVE_SESSION_ROUTES),
  },
  {
    // The player creates their own character (MR-003, character half).
    // `loadChildren` for the same reason as `campanhas/:id` above — see
    // character-editor.routes.ts.
    path: 'campanhas/:id/personagens/novo',
    canActivate: [authGuard],
    loadChildren: () =>
      import('./pages/character-editor/character-editor.routes').then(
        (m) => m.CHARACTER_EDITOR_ROUTES,
      ),
  },
  {
    // The master creates an NPC. `tipo` is `inimigo`, `boss`, `minion` or
    // `historia` (MR-005) — CharacterEditorMode reads it and picks the full
    // or basic form.
    path: 'campanhas/:id/npcs/novo/:tipo',
    canActivate: [authGuard],
    loadChildren: () =>
      import('./pages/character-editor/character-editor.routes').then(
        (m) => m.CHARACTER_EDITOR_ROUTES,
      ),
  },
  {
    // The sheet (MR-004): read-only or editable depending on `can_edit`.
    // `loadChildren` for the same reason as `campanhas/:id` above — see
    // character-sheet.routes.ts.
    path: 'campanhas/:id/personagens/:characterId',
    canActivate: [authGuard],
    loadChildren: () =>
      import('./pages/character-sheet/character-sheet.routes').then(
        (m) => m.CHARACTER_SHEET_ROUTES,
      ),
  },
  {
    // The editor, in edit mode (MR-006 / RN-01: the server refuses this for
    // a player once the sheet is locked).
    path: 'campanhas/:id/personagens/:characterId/editar',
    canActivate: [authGuard],
    loadChildren: () =>
      import('./pages/character-editor/character-editor.routes').then(
        (m) => m.CHARACTER_EDITOR_ROUTES,
      ),
  },
  {
    // The master's gallery (MR-019). A plain `loadComponent`: its clients
    // (GalleryClient, ImageUploader) are `providedIn: 'root'` services that
    // only lazy code imports, so the generated gallery code stays in this
    // route's chunk. The page itself tells a player that only the master
    // sees the gallery (the server refuses them the list).
    path: 'campanhas/:id/galeria',
    canActivate: [authGuard],
    loadComponent: () => import('./pages/gallery/gallery').then((m) => m.GalleryPage),
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
    // Public: the SRD 5.1 CC-BY-4.0 attribution (see creditos.ts), linked
    // from the app footer on every page.
    path: 'creditos',
    loadComponent: () => import('./pages/creditos/creditos').then((m) => m.Creditos),
  },
  {
    path: '**',
    loadComponent: () => import('./pages/not-found/not-found').then((m) => m.NotFound),
  },
];
