import type { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./character-list.component').then((m) => m.CharacterListComponent),
  },
  {
    path: 'novo',
    loadComponent: () =>
      import('./characters.component').then((m) => m.CharactersComponent),
  },
  {
    path: ':id',
    loadComponent: () =>
      import('./characters.component').then((m) => m.CharactersComponent),
  },
];
