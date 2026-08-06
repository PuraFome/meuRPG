import type { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./map-list.component').then((m) => m.MapListComponent),
  },
  {
    path: ':id',
    loadComponent: () =>
      import('./map-view.component').then((m) => m.MapViewComponent),
  },
];
