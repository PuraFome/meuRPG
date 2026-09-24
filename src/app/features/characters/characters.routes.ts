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
    path: 'convites',
    loadComponent: () =>
      import('./character-invites.component').then((m) => m.CharacterInvitesComponent),
  },
  {
    path: 'convidar/:token',
    loadComponent: () =>
      import('./character-join.component').then((m) => m.CharacterJoinComponent),
  },
  {
    path: ':id',
    loadComponent: () =>
      import('./character-detail.component').then((m) => m.CharacterDetailComponent),
  },
];
