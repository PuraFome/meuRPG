import { Component, computed, inject } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { Router, RouterLink } from '@angular/router';

import { AuthService } from '../../core/auth/auth.service';

/**
 * The toolbar's sign-in / account control. Renders exactly one of four
 * things, driven by `AuthService.state` (see its doc comment):
 *
 * - `unknown`: nothing, briefly, while the first `GetMe` is in flight —
 *   avoids a flash of "Entrar" for someone who turns out to be signed in.
 * - `signed-out`: an "Entrar" button.
 * - `signed-in`: the display name (`UpdateProfile`, "Meu perfil"), or
 *   "Minha conta" until the user sets one — never the e-mail, which the app
 *   never has (see AuthUser's doc comment) — as a link to "Meu perfil",
 *   next to a "Sair" button. Two plain buttons rather than a dropdown menu:
 *   `MatMenuModule` pulls in the CDK overlay for two menu items, which is
 *   not worth it in the app shell's initial bundle (see angular.json's
 *   budgets, which this component sits inside as it is always rendered).
 * - `unavailable`: a plain, non-interactive "Servidor indisponível" label.
 *   It intentionally looks nothing like the "Entrar" button, so a signed-in
 *   user does not mistake a server hiccup for having been signed out.
 */
@Component({
  selector: 'app-user-menu',
  imports: [MatButtonModule, MatIconModule, RouterLink],
  templateUrl: './user-menu.html',
  styleUrl: './user-menu.scss',
})
export class UserMenu {
  protected readonly auth = inject(AuthService);
  private readonly router = inject(Router);

  /** "Minha conta" is the neutral fallback until the user sets a display
   * name in "Meu perfil" (see AuthUser). */
  protected readonly accountLabel = computed(() => {
    const state = this.auth.state();
    return state.status === 'signed-in' ? (state.user.displayName ?? 'Minha conta') : '';
  });

  protected signIn(): void {
    this.auth.signIn(this.router.url);
  }

  protected signOut(): void {
    void this.auth.signOut();
  }
}
