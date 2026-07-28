import type { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./map-view.component').then((m) => m.MapViewComponent),
  },
];
