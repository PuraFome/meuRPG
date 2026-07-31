import { APP_INITIALIZER, ApplicationConfig, Injector } from '@angular/core';
import {
  provideRouter,
  withComponentInputBinding,
  withRouterConfig,
} from '@angular/router';
import { provideAnimations } from '@angular/platform-browser/animations';
import { routes } from './app.routes';
import { PersistenceService } from './core/services/persistence.service';

export function initializePersistence(injector: Injector): () => Promise<void> {
  return () => {
    const service = injector.get(PersistenceService);
    service.init();
    return Promise.resolve();
  };
}

export const appConfig: ApplicationConfig = {
  providers: [
    provideRouter(
      routes,
      withComponentInputBinding(),
      withRouterConfig({ onSameUrlNavigation: 'reload' }),
    ),
    provideAnimations(),
    {
      provide: APP_INITIALIZER,
      useFactory: initializePersistence,
      deps: [Injector],
      multi: true,
    },
  ],
};
