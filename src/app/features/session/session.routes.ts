import type { Routes } from '@angular/router';
import { SessionCockpitComponent } from './session-cockpit.component';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./session-cockpit.component').then((m) => m.SessionCockpitComponent),
  },
  {
    path: ':id',
    component: SessionCockpitComponent,
  },
];
