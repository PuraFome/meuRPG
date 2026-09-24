import { inject } from '@angular/core';
import type { HttpInterceptorFn } from '@angular/common/http';

import { environment } from '../../../environments/environment';
import { AuthService } from './auth.service';

/** Attach the bearer token to every call against our own API. */
export const authInterceptor: HttpInterceptorFn = (req, next) => {
  const auth = inject(AuthService);
  const token = auth.token();

  if (token && req.url.startsWith(environment.apiBaseUrl)) {
    return next(
      req.clone({ setHeaders: { Authorization: `Bearer ${token}` } }),
    );
  }
  return next(req);
};
