import type { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./rules.component').then((m) => m.RulesComponent),
  },
];
