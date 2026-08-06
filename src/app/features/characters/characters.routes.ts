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
      import('./character-form.component').then((m) => m.CharacterFormComponent),
  },
  {
    path: ':id',
    loadComponent: () =>
      import('./character-detail.component').then((m) => m.CharacterDetailComponent),
  },
];
