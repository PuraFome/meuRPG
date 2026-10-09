import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

import { reserveActionFailure } from '../../../../core/characters/claim-errors';
import { ClaimsService } from '../../../../core/characters/claims.service';
import { Portrait } from '../../../../shared/portrait/portrait';
import { openSheet } from '../../../../shared/sheet/sheet-host';
import { claimStateLabel, claimTagClass, reservedRowSub } from '../campaign-characters.copy';
import type { CampaignCharacterListItemVm } from '../campaign-characters.types';
import { ClaimLinkSheet, type ClaimLinkData } from './claim-link-sheet';

/** The question a row asks in place: which action, on which character. */
type Question = { readonly id: string; readonly action: 'revoke' | 'return' | 'delete' };

/**
 * "Personagens reservados" on `/campaigns/:id` (MR-049, PM-09 state 3), the master's alone. A reserved character is one the
 * master made, or imported, for a player to take: it has no owner, and no player sees it until one claims it (RN-10). Each
 * row says where its link stands (no link, sent until, used by, revoked) and offers, by state:
 *
 * - "Gerar link para o jogador" / "Gerar novo link": opens the dialog that makes the link and shows it once;
 * - "Revogar o link": asks in place, on the same character, and revokes it;
 * - "Editar": the editor as always, in the master's mode;
 * - "Excluir": asks in place; it deletes the character and so the link it sent;
 * - "Ver ficha" and "Devolver à reserva", for a character a player claimed through a link: the character goes back to
 *   having no owner; the player stays a member.
 *
 * Every question is in place (never a dialog over the list), outlined in the danger colour, with the focus on "Cancelar".
 * When "Revogar o link" loses a race with the player who used it, the row says who took the character and the list is read
 * again. `CampaignCharacters` owns the list; this one reports `changed` after each write so it is read again.
 */
@Component({
  selector: 'app-reserved-characters',
  imports: [MatButtonModule, MatIconModule, Portrait, RouterLink],
  templateUrl: './reserved-characters.html',
  styleUrl: './reserved-characters.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ReservedCharacters {
  private readonly claims = inject(ClaimsService);
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  readonly campaignId = input.required<string>();
  /** The reserved characters, and the ones a player claimed through a link, in the order of the list. */
  readonly rows = input.required<readonly CampaignCharacterListItemVm[]>();

  /** The list changed on the server: read it again. */
  readonly changed = output<void>();

  protected readonly question = signal<Question | null>(null);
  protected readonly busy = signal<string | null>(null);
  /** What the last action said: a failure, or the answer of a revoke that lost to a player. */
  protected readonly notice = signal<{
    readonly tone: 'danger' | 'success';
    readonly title: string;
    readonly text: string;
  } | null>(null);

  protected readonly claimStateLabel = claimStateLabel;
  protected readonly claimTagClass = claimTagClass;
  protected readonly reservedRowSub = reservedRowSub;

  protected isClaimed(c: CampaignCharacterListItemVm): boolean {
    return c.claim?.state === 'used' && c.reserved !== true;
  }

  protected isAsking(c: CampaignCharacterListItemVm, action: Question['action']): boolean {
    const q = this.question();
    return q?.id === c.id && q.action === action;
  }

  protected ask(c: CampaignCharacterListItemVm, action: Question['action']): void {
    this.notice.set(null);
    this.question.set({ id: c.id, action });
    this.focus(`#ask-${c.id} [data-cancel]`);
  }

  /** "Cancelar": the question goes away and the focus goes back to the button that asked it. */
  protected cancel(c: CampaignCharacterListItemVm): void {
    const action = this.question()?.action;
    this.question.set(null);
    this.focus(`#row-${c.id} [data-opener="${action}"]`);
  }

  /** The dialog that makes the link. The list is read again when it closes with a link made. */
  protected openLinkDialog(c: CampaignCharacterListItemVm): void {
    this.notice.set(null);
    const data: ClaimLinkData = {
      campaignId: this.campaignId(),
      characterId: c.id,
      characterName: c.name,
      description: [c.classSummary, c.raceName ?? ''].filter((p) => p !== '').join(', '),
    };
    openSheet<ClaimLinkSheet, ClaimLinkData, boolean>(
      this.dialog,
      this.bottomSheet,
      ClaimLinkSheet,
      {
        data,
        ariaLabel: 'Gerar link para o jogador',
        labelledBy: 'claim-link-t',
        focus: '[data-initial-focus]',
        width: '520px',
      },
    ).subscribe((made) => {
      if (made) {
        this.changed.emit();
      }
    });
  }

  /** Does what the open question asks. */
  protected async confirm(c: CampaignCharacterListItemVm): Promise<void> {
    const q = this.question();
    if (q?.id !== c.id || this.busy() !== null) {
      return;
    }
    this.busy.set(c.id);
    try {
      switch (q.action) {
        case 'revoke':
          await this.claims.revokeLink(this.campaignId(), c.id);
          this.notice.set({
            tone: 'success',
            title: `Link de ${c.name} revogado.`,
            text: 'Quem abrir o link verá apenas “Este link não pode ser usado”. Você pode gerar outro.',
          });
          break;
        case 'return':
          await this.claims.returnToReserve(this.campaignId(), c.id);
          this.notice.set({
            tone: 'success',
            title: `${c.name} voltou para a reserva.`,
            text: 'O personagem está sem dono. Gere um link para mandar a outro jogador.',
          });
          break;
        case 'delete':
          await this.claims.deleteReserved(this.campaignId(), c.id);
          this.notice.set({
            tone: 'success',
            title: `${c.name} foi excluído.`,
            text: 'O link que tinha sido enviado deixou de valer.',
          });
          break;
      }
      this.question.set(null);
      this.changed.emit();
    } catch (err) {
      this.question.set(null);
      const failure = reserveActionFailure(err);
      if (failure.usedBy) {
        const who = failure.usedBy.name;
        this.notice.set({
          tone: 'danger',
          title: who ? `Este link já foi usado por ${who}.` : 'Este link já foi usado.',
          text: `Você não pôde revogá-lo: ${who || 'um jogador'} acabou de assumir ${c.name}. A lista foi atualizada; use “Devolver à reserva” se foi engano.`,
        });
        this.changed.emit();
      } else {
        this.notice.set({ tone: 'danger', title: 'Não deu certo.', text: failure.message });
      }
    } finally {
      this.busy.set(null);
    }
  }

  private focus(selector: string): void {
    afterNextRender(() => this.host.nativeElement.querySelector<HTMLElement>(selector)?.focus(), {
      injector: this.injector,
    });
  }
}
