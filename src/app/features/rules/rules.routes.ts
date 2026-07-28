import type { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./rules-list.component').then((m) => m.RulesListComponent),
    title: 'MeuRPG — Regras',
  },
  {
    path: ':id',
    loadComponent: () =>
      import('./rules-reader.component').then((m) => m.RulesReaderComponent),
    title: 'MeuRPG — Leitor de PDF',
  },
];
