import {
  Component,
  ElementRef,
  OnInit,
  afterNextRender,
  inject,
  input,
  signal,
  computed,
  Injector,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { Code, ConnectError } from '@connectrpc/connect';

import { PendingMember } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CampaignsService } from '../../../core/campaigns/campaigns.service';
import { describeConnectError } from '../../../core/connect/connect-errors';
import { PendingMemberVm, pendingMemberRows } from './pending-members.copy';

/**
 * The master's rows in "Membros" for people who accepted an invite with
 * approval and have not created a character yet (RN-15, MR-024, Samuel's
 * answer to question 24). They are not members yet, so ListMembers does not
 * list them: this asks ListPendingMembers. Only rendered for the master.
 *
 * "Remover" opens the confirmation in place, with focus on "Cancelar" (the
 * safe choice). After a removal, focus goes to the next row, and a status
 * line says who left. If the person created a character meanwhile, the
 * server answers `failed_precondition`: the list is refreshed and the master
 * is pointed to "Esperando aprovação", where that character is.
 */
@Component({
  selector: 'app-pending-members',
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './pending-members.html',
  styleUrl: './pending-members.scss',
})
export class PendingMembers implements OnInit {
  private readonly campaigns = inject(CampaignsService);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  readonly campaignId = input.required<string>();
  readonly campaignName = input.required<string>();

  private readonly pending = signal<PendingMember[]>([]);
  protected readonly rows = computed<PendingMemberVm[]>(() => pendingMemberRows(this.pending()));
  /** The user whose removal is being confirmed. */
  protected readonly confirming = signal<string | null>(null);
  protected readonly removing = signal(false);
  protected readonly status = signal('');
  protected readonly error = signal('');

  ngOnInit(): void {
    void this.load();
  }

  private async load(): Promise<void> {
    try {
      this.pending.set((await this.campaigns.listPendingMembers(this.campaignId())).members);
    } catch (err) {
      this.error.set(
        describeConnectError(err, {
          [Code.PermissionDenied]: 'Só o mestre da campanha vê quem está sem personagem.',
          [Code.Unavailable]:
            'Não foi possível ver quem está sem personagem. Tente de novo em instantes.',
        }),
      );
    }
  }

  protected ask(userId: string): void {
    this.status.set('');
    this.error.set('');
    this.confirming.set(userId);
    this.focusAfterRender(`#remove-title-${userId} ~ .confirm__actions .cancel-button`);
  }

  protected cancel(userId: string): void {
    this.confirming.set(null);
    this.focusAfterRender(`.remove-button[data-user-id="${userId}"]`);
  }

  protected async remove(member: PendingMemberVm): Promise<void> {
    const rows = this.rows();
    const next = rows[rows.findIndex((r) => r.userId === member.userId) + 1];
    this.removing.set(true);
    try {
      await this.campaigns.removePendingMember(this.campaignId(), member.userId);
      this.confirming.set(null);
      this.pending.update((list) => list.filter((p) => p.userId !== member.userId));
      this.status.set(`${member.name} saiu de ${this.campaignName()}.`);
      this.focusAfterRender(next ? `.remove-button[data-user-id="${next.userId}"]` : '.status');
    } catch (err) {
      this.confirming.set(null);
      const code = ConnectError.from(err, Code.Unavailable).code;
      if (code === Code.FailedPrecondition) {
        this.error.set(
          `${member.name} acabou de criar o personagem: aprove ou recuse em Esperando aprovação.`,
        );
        await this.load();
      } else if (code === Code.NotFound) {
        // Already gone (the daily clean-up, or another tab): the list was stale.
        this.error.set(`${member.name} já não está na campanha.`);
        await this.load();
      } else {
        this.error.set(
          describeConnectError(err, {
            [Code.PermissionDenied]: 'Só o mestre da campanha pode remover quem está esperando.',
          }),
        );
      }
    } finally {
      this.removing.set(false);
    }
  }

  private focusAfterRender(selector: string): void {
    afterNextRender(() => this.host.nativeElement.querySelector<HTMLElement>(selector)?.focus(), {
      injector: this.injector,
    });
  }
}
