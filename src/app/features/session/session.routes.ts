import type { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./session.component').then((m) => m.SessionComponent),
  },
];
