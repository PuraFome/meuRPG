import type { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./session-cockpit.component').then((m) => m.SessionCockpitComponent),
  },
];
