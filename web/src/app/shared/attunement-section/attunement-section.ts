import { NgTemplateOutlet } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  computed,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';

import {
  type AttunementEntryVm,
  type AttunementVm,
  AttunementClient,
  isRestrictedRefusal,
} from '../../core/characters/attunement';
import { describeCharacterError } from '../../core/characters/character-errors';

import { type AttunementRowVm, type AttunementViewer, sectionOf } from './attunement-rows';

let nextId = 0;

/**
 * "Sintonização": the items that ask for attunement and the ones the character is attuned to, each tap one `SetAttunement`
 * that takes effect at once (no "Salvar"). It lives in two places: the short-rest sheet, while the owner's window is open,
 * and the dialog "Mudar" opens. The server says who meets which restriction and refuses what it must; the section only
 * words it and marks the buttons it knows will be refused as `aria-disabled`, with the reason under the row.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-attunement-section',
  imports: [NgTemplateOutlet, MatButtonModule, MatIconModule, MatProgressSpinnerModule],
  templateUrl: './attunement-section.html',
  styleUrl: './attunement-section.scss',
})
export class AttunementSection {
  private readonly client = inject(AttunementClient);

  readonly attunement = input.required<AttunementVm>();
  readonly viewer = input.required<AttunementViewer>();
  /** The character's name, for the master's words. */
  readonly characterName = input('');
  /** The owner's window is open (`CharacterVitals.attunement_open`). */
  readonly windowOpen = input(false);
  readonly dead = input(false);
  /** The section sits under a title of its own (the sheet "Sintonização"): no heading of its own, the count stays. */
  readonly bare = input(false);
  readonly campaignId = input.required<string>();
  readonly characterId = input.required<string>();
  /** The revision of the sheet the caller holds; a stale one is refused by the server. */
  readonly revision = input.required<number>();
  /** The sheet changed (an attunement was set): the page reads it again. */
  readonly changed = output<void>();
  /** The server refused: what the section holds (the revision, the entries) may be stale, so the page reads it again. */
  readonly refused = output<void>();

  protected readonly headingId = `att-heading-${nextId++}`;
  protected readonly limitId = `${this.headingId}-limit`;
  protected readonly busyId = signal<string | null>(null);
  protected readonly status = signal('');
  protected readonly error = signal('');

  protected readonly section = computed(() =>
    sectionOf(this.attunement(), {
      viewer: this.viewer(),
      name: this.characterName(),
      open: this.windowOpen(),
      dead: this.dead(),
    }),
  );

  protected lowerFirst(text: string): string {
    return text.charAt(0).toLowerCase() + text.slice(1);
  }

  /** What follows "Limite de 3 itens." in the limit line. */
  protected readonly limitRest = computed(
    () => ` ${this.section().limitLine.replace('Limite de 3 itens. ', '')}`,
  );

  protected reasonId(r: AttunementRowVm): string {
    return `${this.headingId}-why-${r.entry.id.replace(/[^a-z0-9]/gi, '-')}`;
  }

  protected icon(e: AttunementEntryVm): string {
    return e.kind === 'piece' ? 'swords' : 'diamond';
  }

  /** A tap on "Sintonizar" or "Encerrar": one call, then the status line or the alert. A button the section knows will be
   * refused does nothing; the server would answer with the same reason. */
  protected async tap(r: AttunementRowVm): Promise<void> {
    if (r.button !== 'on' || this.busyId() !== null) {
      return;
    }
    const attuned = r.verb === 'Sintonizar';
    this.busyId.set(r.entry.id);
    this.error.set('');
    this.status.set('');
    try {
      await this.client.set(
        this.campaignId(),
        this.characterId(),
        r.entry.id,
        attuned,
        this.revision(),
      );
      this.changed.emit();
      const count = this.attunement().attuned + (r.entry.asks ? (attuned ? 1 : -1) : 0);
      this.status.set(
        `${r.entry.namePt} ${attuned ? 'sintonizado' : 'desfeita a sintonização'}. Sintonizados: ${count} de 3.`,
      );
    } catch (err) {
      this.refused.emit();
      const why = isRestrictedRefusal(err) && r.reason ? r.reason : describeCharacterError(err);
      this.error.set(
        `Não foi possível ${attuned ? 'sintonizar' : 'encerrar'} ${r.entry.namePt}. ${why}`,
      );
    } finally {
      this.busyId.set(null);
    }
  }
}
