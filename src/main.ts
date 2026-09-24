import { bootstrapApplication } from '@angular/platform-browser';
import { appConfig } from './app/app.config';
import { App } from './app/app';

const AUTH_TOKEN_KEY = 'meurpg.auth.token';
const PENDING_REDIRECT_KEY = 'meurpg.auth.pendingRedirect';

// The OAuth callback lands on `#token=<opaque>`. With hash-location routing
// Angular would otherwise treat that fragment as a route, so persist it and
// normalise the URL before the app bootstraps. A stashed route (e.g. an invite
// link that bounced through /login) is resumed instead of the home page.
function consumeLoginFragment(): void {
  const match = /^#token=([A-Za-z0-9_-]+)$/.exec(window.location.hash);
  if (!match) {
    return;
  }
  localStorage.setItem(AUTH_TOKEN_KEY, match[1]);
  const pending = localStorage.getItem(PENDING_REDIRECT_KEY);
  localStorage.removeItem(PENDING_REDIRECT_KEY);
  const target = pending?.startsWith('/') ? `#${pending}` : '#/';
  history.replaceState(
    null,
    '',
    window.location.pathname + window.location.search + target,
  );
}

consumeLoginFragment();

bootstrapApplication(App, appConfig).catch((err) => console.error(err));
