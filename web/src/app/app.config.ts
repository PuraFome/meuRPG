import {
  ApplicationConfig,
  inject,
  provideAppInitializer,
  provideBrowserGlobalErrorListeners,
} from '@angular/core';
import { MatIconRegistry } from '@angular/material/icon';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { provideRouter } from '@angular/router';

import { routes } from './app.routes';

export const appConfig: ApplicationConfig = {
  providers: [
    provideBrowserGlobalErrorListeners(),
    provideRouter(routes),
    // Loads the animations runtime lazily, only if a component (e.g. a
    // Material overlay or ripple) actually asks for it.
    provideAnimationsAsync(),
    // <mat-icon>name</mat-icon> defaults to Google's "Material Icons"
    // ligature font, which this app does not ship (no Google Fonts — see
    // docs/privacidade.md). Point it at the self-hosted Material Symbols
    // font instead (index.html's <link>, copied into the build by
    // angular.json's assets entry).
    provideAppInitializer(() => {
      inject(MatIconRegistry).setDefaultFontSetClass('material-symbols-outlined');
    }),
  ],
};
