import { Component, input } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

/**
 * The session link's dead ends (artboard E5-07, D3):
 * - `no-access`: the server answered `not_found` (not a member, a pending
 *   member, or no such campaign). "Peça um convite ao mestre", never the
 *   campaign's name: the link must not tell a stranger what's behind it.
 * - `no-session`: a member, but no session is open. The campaign's name,
 *   and back to the campaign. When the master starts one, the page opens
 *   it by itself (the app's 30-second poll sees it), so the notice says
 *   "a sessão abre aqui" instead of the artboard's "o aviso aparece aqui".
 * - `ended`: the session ended while the page was open (`session_ended`,
 *   or a correction refused with `NO_OPEN_SESSION`).
 *
 * Each has one filled action, its only one.
 */
@Component({
  selector: 'app-session-blocked',
  imports: [MatButtonModule, MatIconModule, RouterLink],
  templateUrl: './session-blocked.html',
  styleUrl: './session-blocked.scss',
})
export class SessionBlocked {
  readonly kind = input.required<'no-access' | 'no-session' | 'ended'>();
  readonly campaignId = input('');
  readonly campaignName = input('');
  readonly sessionNumber = input(0);
}
