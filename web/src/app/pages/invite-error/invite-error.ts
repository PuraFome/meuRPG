import { Component, computed, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { map } from 'rxjs';

import { describeInviteErrorCode } from '../../core/campaigns/invite-errors';

/**
 * `describeInviteErrorCode`'s own message already tells the visitor what to
 * do next for these reasons (check the link, ask the mestre, try again
 * later) — see core/campaigns/invite-errors.ts. Every other reason gets
 * this page's generic follow-up instead, so nobody reads the same
 * instruction twice.
 */
const REASONS_WITH_OWN_ADVICE = new Set(['used_up', 'not_found', 'unavailable']);

/**
 * "/invite/error?reason=<code>" (public): where the server redirects after
 * the sign-in-through-invite flow when the invite could not be accepted —
 * `expired`, `revoked`, `used_up`, `not_found` or `invalid`
 * (docs/architecture.md#web-app-web).
 */
@Component({
  selector: 'app-invite-error',
  imports: [MatButtonModule, MatIconModule, RouterLink],
  templateUrl: './invite-error.html',
  styleUrl: './invite-error.scss',
})
export class InviteError {
  private readonly route = inject(ActivatedRoute);

  private readonly reason = toSignal(
    this.route.queryParamMap.pipe(map((params) => params.get('reason'))),
    { initialValue: null },
  );

  protected readonly message = computed(() => describeInviteErrorCode(this.reason()));

  protected readonly nextStep = computed(() =>
    REASONS_WITH_OWN_ADVICE.has(this.reason() ?? '')
      ? null
      : 'Peça um novo link a quem te convidou.',
  );
}
