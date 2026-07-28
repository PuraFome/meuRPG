import type { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./map.component').then((m) => m.MapComponent),
  },
];
