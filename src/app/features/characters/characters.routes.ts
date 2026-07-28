import type { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./characters.component').then((m) => m.CharactersComponent),
  },
];
