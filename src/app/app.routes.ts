import type { Routes } from '@angular/router';
import { ShellComponent } from './layout/shell.component';
import { HomeComponent } from './home.component';

export const routes: Routes = [
  {
    path: '',
    component: ShellComponent,
    children: [
      {
        path: '',
        component: HomeComponent,
        title: 'MeuRPG — Início',
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
        title: 'MeuRPG — Campanha',
      },
      {
        path: 'galeria',
        loadChildren: () =>
          import('./features/gallery/gallery.routes').then((m) => m.routes),
        title: 'MeuRPG — Galeria',
      },
      {
        path: 'regras',
        loadChildren: () =>
          import('./features/rules/rules.routes').then((m) => m.routes),
        title: 'MeuRPG — Regras',
      },
      {
        path: 'sessao',
        loadChildren: () =>
          import('./features/session/session.routes').then((m) => m.routes),
        title: 'MeuRPG — Sessão',
      },
      {
        path: 'mapa',
        loadChildren: () =>
          import('./features/map/map.routes').then((m) => m.routes),
        title: 'MeuRPG — Mapa',
      },
    ],
  },
  {
    path: '**',
    redirectTo: '',
  },
];
