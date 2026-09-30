import { Component, inject } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { ActivatedRoute, Router } from '@angular/router';

import { AuthService } from '../../core/auth/auth.service';

/**
 * Where `authGuard` sends a guarded navigation when `AuthService` is
 * `unavailable` (server or database not answering) — see the guard's doc
 * comment for why that is not the same as bouncing through sign-in.
 *
 * Keeps the originally requested path in `return_to`, so "Tentar de novo"
 * can go straight there once the retry succeeds, instead of dropping the
 * user on the home page.
 */
@Component({
  selector: 'app-server-unavailable',
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './server-unavailable.html',
  styleUrl: './server-unavailable.scss',
})
export class ServerUnavailable {
  protected readonly auth = inject(AuthService);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);

  protected async retry(): Promise<void> {
    await this.auth.refresh();
    if (this.auth.state().status === 'unavailable') {
      return;
    }
    const returnTo = this.route.snapshot.queryParamMap.get('return_to');
    const safe =
      returnTo && returnTo.startsWith('/') && !returnTo.startsWith('//') ? returnTo : '/';
    await this.router.navigateByUrl(safe);
  }
}
