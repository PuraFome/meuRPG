import { APP_INITIALIZER, ApplicationConfig, Injector } from '@angular/core';
import { provideHttpClient, withInterceptors } from '@angular/common/http';
import {
  provideRouter,
  withComponentInputBinding,
  withHashLocation,
  withRouterConfig,
} from '@angular/router';
import { provideAnimations } from '@angular/platform-browser/animations';
import { routes } from './app.routes';
import { PersistenceService } from './core/services/persistence.service';
import { AuthService } from './core/auth/auth.service';
import { authInterceptor } from './core/auth/auth.interceptor';

export function initializePersistence(injector: Injector): () => Promise<void> {
  return () => {
    const service = injector.get(PersistenceService);
    service.init();
    return Promise.resolve();
  };
}

export function initializeAuth(auth: AuthService): () => Promise<void> {
  return () => auth.ready();
}

export const appConfig: ApplicationConfig = {
  providers: [
    provideRouter(
      routes,
      withComponentInputBinding(),
      withHashLocation(),
      withRouterConfig({ onSameUrlNavigation: 'reload' }),
    ),
    provideAnimations(),
    provideHttpClient(withInterceptors([authInterceptor])),
    {
      provide: APP_INITIALIZER,
      useFactory: initializeAuth,
      deps: [AuthService],
      multi: true,
    },
    {
      provide: APP_INITIALIZER,
      useFactory: initializePersistence,
      deps: [Injector],
      multi: true,
    },
  ],
};
