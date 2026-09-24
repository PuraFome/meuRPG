import type { Routes } from '@angular/router';
import { ShellComponent } from './layout/shell.component';
import { HomeComponent } from './home.component';
import { authGuard, loginGuard } from './core/auth/auth.guard';
import { visitorGuard } from './core/auth/visitor.guard';

export const routes: Routes = [
  {
    path: 'login',
    loadComponent: () =>
      import('./features/login/login.component').then((m) => m.LoginComponent),
    canActivate: [loginGuard],
    title: 'MeuRPG — Entrar',
  },
  {
    path: '',
    component: ShellComponent,
    children: [
      {
        path: '',
        component: HomeComponent,
        canActivate: [authGuard, visitorGuard],
        title: 'MeuRPG — Início',
      },
      {
        path: 'perfil',
        loadComponent: () =>
          import('./features/perfil/perfil.component').then(
            (m) => m.PerfilComponent,
          ),
        canActivate: [authGuard, visitorGuard],
        title: 'MeuRPG — Perfil',
      },
      {
        path: 'personagens',
        loadChildren: () =>
          import('./features/characters/characters.routes').then((m) => m.routes),
        title: 'MeuRPG — Personagens',
      },
      {
        path: 'campanha',
        loadChildren: () =>
          import('./features/campaign/campaign.routes').then((m) => m.routes),
        canActivate: [authGuard, visitorGuard],
        title: 'MeuRPG — Campanha',
      },
      {
        path: 'galeria',
        loadChildren: () =>
          import('./features/gallery/gallery.routes').then((m) => m.routes),
        canActivate: [authGuard, visitorGuard],
        title: 'MeuRPG — Galeria',
      },
      {
        path: 'regras',
        loadChildren: () =>
          import('./features/rules/rules.routes').then((m) => m.routes),
        canActivate: [authGuard, visitorGuard],
        title: 'MeuRPG — Regras',
      },
      {
        path: 'sessao',
        loadChildren: () =>
          import('./features/session/session.routes').then((m) => m.routes),
        canActivate: [authGuard, visitorGuard],
        title: 'MeuRPG — Sessão',
      },
      {
        path: 'mapa',
        loadChildren: () =>
          import('./features/map/map.routes').then((m) => m.routes),
        canActivate: [authGuard, visitorGuard],
        title: 'MeuRPG — Mapa',
      },
    ],
  },
  {
    path: '**',
    redirectTo: '',
  },
];
