import { Component, computed, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { map } from 'rxjs';

import { describeInviteErrorCode } from '../../core/campaigns/invite-errors';

/**
 * "/convite/erro?motivo=<code>" (public): where the server redirects after
 * the sign-in-through-invite flow when the invite could not be accepted —
 * `expired`, `revoked`, `used_up`, `not_found` or `invalid`
 * (docs/arquitetura.md#frontend-web).
 */
@Component({
  selector: 'app-invite-error',
  imports: [MatButtonModule, MatCardModule, RouterLink],
  templateUrl: './invite-error.html',
  styleUrl: './invite-error.scss',
})
export class InviteError {
  private readonly route = inject(ActivatedRoute);

  private readonly motivo = toSignal(
    this.route.queryParamMap.pipe(map((params) => params.get('motivo'))),
    { initialValue: null },
  );

  protected readonly message = computed(() => describeInviteErrorCode(this.motivo()));
}
