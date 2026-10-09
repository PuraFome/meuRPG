import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  effect,
  inject,
  signal,
  viewChild,
} from '@angular/core';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { describeCharacterError } from '../../../../core/characters/character-errors';
import { ClaimsService } from '../../../../core/characters/claims.service';
import { SheetFrame } from '../../../../shared/sheet/sheet-frame/sheet-frame';
import { injectSheet } from '../../../../shared/sheet/sheet-host';
import { dayMonthTime } from '../campaign-characters.copy';

/** What the list hands the dialog. */
export interface ClaimLinkData {
  readonly campaignId: string;
  readonly characterId: string;
  readonly characterName: string;
  /** "Monge 5, Humano": what the line under the title says about the character. */
  readonly description: string;
}

const DAY_MS = 86_400_000;
const WEEK = 7;
const MONTH = 30;
/** The days a link may work: 1, 7 (the default) or 30. */
export const VALIDITY_CHOICES = [1, WEEK, MONTH] as const;
const DEFAULT_VALIDITY = WEEK;

type Step =
  | { readonly status: 'choose' }
  | { readonly status: 'making' }
  | {
      readonly status: 'shown';
      readonly link: string;
      readonly days: number;
      readonly expiresAt: Date;
    };

/**
 * "Gerar link para o jogador" (MR-049, PM-09 state 4): a dialog on a computer and a sheet on a phone. The master picks how
 * long the link works (1, 7 or 30 days; 7 by default), the server makes it, and the dialog shows it once, whole, in a
 * read-only field that wraps, with "Copiar link". The validity cannot change after the link is shown, and the server never
 * shows the link again: closing the dialog loses it, and a new one revokes this one. "Revogar o link" is not here, it is on
 * the character's row.
 *
 * The link is `<origin>/claim#t=<token>`: the secret sits after the `#`, which the browser never sends to a server.
 */
@Component({
  selector: 'app-claim-link-sheet',
  imports: [MatButtonModule, MatIconModule, SheetFrame],
  templateUrl: './claim-link-sheet.html',
  styleUrl: './claim-link-sheet.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ClaimLinkSheet {
  private readonly claims = inject(ClaimsService);
  private readonly sheet = injectSheet<ClaimLinkData, boolean>();
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;
  protected readonly choices = VALIDITY_CHOICES;
  protected readonly dayMonthTime = dayMonthTime;
  /** " (Monge 5, Humano)": what follows the name, or nothing. */
  protected readonly aboutIt = this.data.description ? ` (${this.data.description})` : '';

  protected readonly step = signal<Step>({ status: 'choose' });
  protected readonly days = signal<number>(DEFAULT_VALIDITY);
  protected readonly error = signal('');
  /** "Link copiado.": the live region's words after "Copiar link". */
  protected readonly copyNote = signal('');

  private readonly linkField = viewChild<ElementRef<HTMLTextAreaElement>>('linkField');
  /** Whether a link was made: the list reads again when the dialog closes. */
  private made = false;

  /** A request in the air: Esc and the backdrop do not close the dialog under it, so the answer is never lost. */
  protected readonly lockWhileMaking = effect(() =>
    this.sheet.lock(this.step().status === 'making'),
  );

  protected pick(days: number): void {
    this.days.set(days);
  }

  protected async make(): Promise<void> {
    if (this.step().status === 'making') {
      return;
    }
    const days = this.days();
    this.error.set('');
    this.step.set({ status: 'making' });
    try {
      const res = await this.claims.createLink(this.data.campaignId, this.data.characterId, days);
      this.made = true;
      this.step.set({
        status: 'shown',
        link: `${window.location.origin}/claim#t=${res.token}`,
        days,
        expiresAt: res.expiresAt
          ? timestampDate(res.expiresAt)
          : new Date(Date.now() + days * DAY_MS),
      });
      afterNextRender(
        () => this.host.nativeElement.querySelector<HTMLElement>('[data-copy]')?.focus(),
        { injector: this.injector },
      );
    } catch (err) {
      this.error.set(describeCharacterError(err));
      this.step.set({ status: 'choose' });
    }
  }

  protected async copy(link: string): Promise<void> {
    try {
      await navigator.clipboard.writeText(link);
      this.copyNote.set('Link copiado.');
    } catch {
      this.linkField()?.nativeElement.select();
      this.copyNote.set(
        'Não foi possível copiar sozinho. O link está selecionado: copie com o teclado.',
      );
    }
  }

  protected selectAll(event: Event): void {
    (event.target as HTMLTextAreaElement).select();
  }

  protected close(): void {
    this.sheet.close(this.made);
  }

  /** "dia" or "dias". */
  protected daysWord(n: number): string {
    return n === 1 ? 'dia' : 'dias';
  }
}
