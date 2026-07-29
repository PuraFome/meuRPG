import type { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./campaign.component').then((m) => m.CampaignComponent),
  },
];
