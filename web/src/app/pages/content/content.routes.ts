import { Routes } from '@angular/router';

/**
 * "Conteúdo da mesa" (MR-025, RN-23): the list, then one entry, new or existing. Lazy from `app.routes.ts` by
 * `loadChildren`, so the editors, the effect picker and the table content client stay out of the initial bundle.
 * `options` is "Opções para os jogadores" (the master's switches, RN-23); `new/:kind` comes before `entries/:key`; the key of an entry ("race:corujeiro@mesa") is one path segment, encoded.
 */
export const CONTENT_ROUTES: Routes = [
  {
    path: '',
    title: 'Conteúdo da mesa',
    loadComponent: () => import('./content-list/content-list').then((m) => m.ContentList),
  },
  {
    path: 'options',
    title: 'Opções para os jogadores',
    loadComponent: () => import('./options/options').then((m) => m.ContentOptions),
  },
  {
    path: 'new/:kind',
    title: 'Nova entrada da mesa',
    loadComponent: () => import('./content-entry/content-entry').then((m) => m.ContentEntry),
  },
  {
    path: 'entries/:key',
    title: 'Entrada da mesa',
    loadComponent: () => import('./content-entry/content-entry').then((m) => m.ContentEntry),
  },
];
