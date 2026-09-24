import { bootstrapApplication } from '@angular/platform-browser';
import { appConfig } from './app/app.config';
import { App } from './app/app';

const AUTH_TOKEN_KEY = 'meurpg.auth.token';

// The OAuth callback lands on `#token=<opaque>`. With hash-location routing
// Angular would otherwise treat that fragment as a route, so persist it and
// normalise the URL before the app bootstraps.
function consumeLoginFragment(): void {
  const match = /^#token=([A-Za-z0-9_-]+)$/.exec(window.location.hash);
  if (!match) {
    return;
  }
  localStorage.setItem(AUTH_TOKEN_KEY, match[1]);
  history.replaceState(
    null,
    '',
    window.location.pathname + window.location.search + '#/',
  );
}

consumeLoginFragment();

bootstrapApplication(App, appConfig).catch((err) => console.error(err));
