import { ApplicationConfig } from '@angular/core';
import {
  provideRouter,
  withComponentInputBinding,
  withRouterConfig,
} from '@angular/router';
import { provideAnimations } from '@angular/platform-browser/animations';
import { routes } from './app.routes';

export const appConfig: ApplicationConfig = {
  providers: [
    provideRouter(
      routes,
      withComponentInputBinding(),
      withRouterConfig({ onSameUrlNavigation: 'reload' }),
    ),
    provideAnimations(),
  ],
};
