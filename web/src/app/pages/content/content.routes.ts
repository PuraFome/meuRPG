import { Routes } from '@angular/router';

/**
 * "Conteúdo da mesa" (MR-025, RN-23): the list, then one entry, new or existing. Lazy from `app.routes.ts` by
 * `loadChildren`, so the editors, the effect picker and the table content client stay out of the initial bundle.
 * `novo/:tipo` comes before `entrada/:key`; the key of an entry ("race:corujeiro@mesa") is one path segment, encoded.
 */
export const CONTENT_ROUTES: Routes = [
  { path: '', loadComponent: () => import('./content-list/content-list').then((m) => m.ContentList) },
  { path: 'novo/:tipo', loadComponent: () => import('./content-entry/content-entry').then((m) => m.ContentEntry) },
  { path: 'entrada/:key', loadComponent: () => import('./content-entry/content-entry').then((m) => m.ContentEntry) },
];
