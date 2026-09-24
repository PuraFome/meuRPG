import type { Routes } from '@angular/router';
import { authGuard } from '../../core/auth/auth.guard';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./character-list.component').then((m) => m.CharacterListComponent),
    canActivate: [authGuard],
  },
  {
    path: 'novo',
    loadComponent: () =>
      import('./character-form.component').then((m) => m.CharacterFormComponent),
    canActivate: [authGuard],
  },
  {
    path: 'convites',
    loadComponent: () =>
      import('./character-invites.component').then((m) => m.CharacterInvitesComponent),
    canActivate: [authGuard],
  },
  {
    path: 'convidar/:token',
    loadComponent: () =>
      import('./character-join.component').then((m) => m.CharacterJoinComponent),
    canActivate: [authGuard],
  },
  {
    path: ':id',
    loadComponent: () =>
      import('./character-detail.component').then((m) => m.CharacterDetailComponent),
    canActivate: [authGuard],
  },
];
