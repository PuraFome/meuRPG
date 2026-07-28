import type { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./gallery.component').then((m) => m.GalleryComponent),
  },
];
