import { Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

/** Whether "Copiar link" worked, failed, or was not pressed yet. */
export type CopyStatus = 'idle' | 'copied' | 'error';

/**
 * The just-created invite's link, shown exactly once (CreateInvite: "this
 * is the only time the server ever returns it"), inside "Convites". A box
 * in the confirmation colours, so it is the obvious thing on the panel,
 * with the link in a field of its own (one tap selects all of it) and
 * "Copiar link". `CampaignInvites` owns the state; this only shows it.
 *
 * The e2e tests read the link from `.invite-reveal__link`'s text.
 */
@Component({
  selector: 'app-invite-reveal',
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './invite-reveal.html',
  styleUrl: './invite-reveal.scss',
})
export class InviteReveal {
  readonly link = input.required<string>();
  /** The invite requires approval (RN-15): say what whoever uses it sees. */
  readonly requiresApproval = input(false);
  readonly copyStatus = input<CopyStatus>('idle');

  readonly copy = output<void>();
  readonly dismiss = output<void>();
}
