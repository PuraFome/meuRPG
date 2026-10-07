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
    // The guided level-up of a locked sheet (MR-040, RN-01's exception, RN-12): the owning
    // player's own page; the server answers for anyone else, and the page says so.
    path: 'campanhas/:id/personagens/:characterId/subir-de-nivel',
    canActivate: [authGuard],
    loadComponent: () => import('./pages/level-up/level-up').then((m) => m.LevelUpPage),
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
    // The master's puzzles (MR-038, E10-06): make one, edit one. The list is a panel on the campaign page; showing a puzzle
    // and playing it live belong to the session page.
    path: 'campanhas/:id/quebra-cabecas/novo',
    canActivate: [authGuard],
    loadComponent: () => import('./pages/puzzles/puzzle-form/puzzle-form').then((m) => m.PuzzleForm),
  },
  {
    path: 'campanhas/:id/quebra-cabecas/:puzzleId/editar',
    canActivate: [authGuard],
    loadComponent: () => import('./pages/puzzles/puzzle-form/puzzle-form').then((m) => m.PuzzleForm),
  },
  {
    // The bestiary (MR-042, E10-08), master only: the SRD's 334 creatures, then one creature's
    // stat block with "Criar NPC". The pages tell a player so (the SRD is public, the app shows
    // it to the master). The creature's route is after the list's, a plain `loadComponent` for
    // each, like the gallery.
    path: 'campanhas/:id/bestiario',
    canActivate: [authGuard],
    loadComponent: () => import('./pages/bestiary/bestiary-list/bestiary-list').then((m) => m.BestiaryList),
  },
  {
    path: 'campanhas/:id/bestiario/:slug',
    canActivate: [authGuard],
    loadComponent: () =>
      import('./pages/bestiary/bestiary-creature/bestiary-creature').then((m) => m.BestiaryCreature),
  },
  {
    // The encounter builder (MR-043, RN-29, E10-09), master only: the party's budget, the creatures, "Gerar encontro" and "Guardar no
    // ponto de batalha". `?mapa=&ponto=` (from a battle point of the editor) brings that point's saved encounter into the draft and preselects the point in "Guardar". A lazy route like the bestiary.
    path: 'campanhas/:id/encontros',
    canActivate: [authGuard],
    loadComponent: () => import('./pages/encounters/encounter-builder/encounter-builder').then((m) => m.EncounterBuilder),
  },
  {
    // "Magias" (MR-045, RN-23): the players' spell reference, for every active member, the master too.
    path: 'campanhas/:id/magias',
    canActivate: [authGuard],
    loadComponent: () => import('./pages/spells/spells').then((m) => m.Spells),
  },
  {
    // The campaign document (MR-018), master only; the page tells a player
    // so. Leaving edit mode with unsaved text asks first (the guard lives
    // with the page, the page decides, so this file imports nothing of it).
    path: 'campanhas/:id/documento',
    canActivate: [authGuard],
    canDeactivate: [(page: { confirmLeave(): boolean | Promise<boolean> }) => page.confirmLeave()],
    loadComponent: () =>
      import('./pages/campaign-document/campaign-document').then((m) => m.CampaignDocumentPage),
  },
  {
    // "Regras da mesa" (MR-025, RN-24, RN-09), master only: the page says so to a player.
    path: 'campanhas/:id/regras',
    canActivate: [authGuard],
    loadComponent: () => import('./pages/table-rules/table-rules').then((m) => m.TableRulesPage),
  },
  {
    // "Conteúdo da mesa" (MR-025, RN-23, E10-01): the table's own classes, races, backgrounds and spells; the master's list and
    // editors, and the players' read view. `loadChildren` so the editors stay out of the eager bundle.
    path: 'campanhas/:id/conteudo',
    canActivate: [authGuard],
    loadChildren: () => import('./pages/content/content.routes').then((m) => m.CONTENT_ROUTES),
  },
  {
    // New map (MR-008, E5-31). Before `mapas/:mapId`, so "novo" is not read
    // as a map's ID. Plain `loadComponent`, like the gallery: the clients
    // are root services that only lazy code imports.
    path: 'campanhas/:id/mapas/novo',
    canActivate: [authGuard],
    loadComponent: () => import('./pages/maps/map-new/map-new').then((m) => m.MapNew),
  },
  {
    // "Gerar masmorra" (MR-010, E10-05), master only: the options, the server's preview and "Criar o mapa". Before `mapas/:mapId`, so
    // "masmorra" is not read as a map's ID. Plain `loadComponent`, like "Novo mapa": its client is a root service that only lazy code imports.
    path: 'campanhas/:id/mapas/masmorra',
    canActivate: [authGuard],
    loadComponent: () => import('./pages/maps/dungeon-new/dungeon-new').then((m) => m.DungeonNew),
  },
  {
    // The map's battle grid (MR-013, E6-02), master only: the squares of
    // 1,5 m that a combat measures movement in.
    path: 'campanhas/:id/mapas/:mapId/grade',
    canActivate: [authGuard],
    loadComponent: () => import('./pages/maps/map-grid/map-grid').then((m) => m.MapGrid),
  },
  {
    // Print the map with its grid to scale (MR-033, E8-12), master only: the
    // page says so to a player. Before `mapas/:mapId` is not needed (more
    // segments), but it stays next to `grade`, its sibling.
    path: 'campanhas/:id/mapas/:mapId/imprimir',
    canActivate: [authGuard],
    loadComponent: () => import('./pages/maps/map-print/map-print').then((m) => m.MapPrint),
  },
  {
    // One map (MR-008, MR-009): the master's editor, or the player's
    // viewer; the page picks by role and screen size.
    path: 'campanhas/:id/mapas/:mapId',
    canActivate: [authGuard],
    // The master's editor may hold strokes the server has not taken yet: it sends them, or asks, before the page goes.
    canDeactivate: [(page: { confirmLeave(): boolean | Promise<boolean> }) => page.confirmLeave()],
    loadComponent: () => import('./pages/maps/map-page/map-page').then((m) => m.MapPage),
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
