import type { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./campaign-tree.component').then((m) => m.CampaignTreeComponent),
  },
];
