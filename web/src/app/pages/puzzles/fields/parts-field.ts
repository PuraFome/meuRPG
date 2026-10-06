import { ChangeDetectionStrategy, Component, ElementRef, Injector, OnInit, afterNextRender, computed, inject, input, output, signal, viewChild, viewChildren } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';

import { CharacterKind } from '../../../../gen/meurpg/characters/v1/characters_pb';
import { type PartDraft, PARTS_MAX, PART_MAX, textLength } from '../../../core/puzzles/puzzle-draft';
import { RosterClient } from '../../../core/maps/roster-client';

interface Who {
  readonly id: string;
  readonly name: string;
}

/**
 * "Informação dividida" of every form (MR-038, RN-27, E10-12 state 4): the master writes a clue in parts and gives each one to a player's
 * character, at most 8 and one per character. Each phone shows only its own part, and the others only learn who has one. A part with
 * no owner is "Sem dono": nobody reads it, and the master gives it before showing the puzzle. The characters are the campaign's player
 * characters; the server checks that each is a living one of the party and refuses the part in words (`errors`, by part).
 */
@Component({
  selector: 'app-parts-field',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatFormFieldModule, MatIconModule, MatInputModule],
  template: `
    <section class="pf" aria-labelledby="pf-title">
      <h3 class="pf__title" id="pf-title">Informação dividida</h3>
      <p class="pf__help">Escreva a pista em partes e dê cada uma a um jogador. Cada celular mostra só a parte dele.</p>
      @if (failed()) {
        <p class="pf__bad" role="alert">Não deu para ler os personagens. Volte e tente de novo.</p>
      }
      @for (part of parts(); track $index; let at = $index) {
        @let n = at + 1;
        @let err = errors()[at];
        <div class="part">
          <div class="part__head">
            <span class="part__badge" aria-hidden="true">{{ badge(part) }}</span>
            <h4 class="part__title">Parte {{ n }}</h4>
            <span class="part__for">{{ owner(part) }}</span>
            <button type="button" class="part__remove" [attr.aria-label]="'Remover a parte ' + n" (click)="remove(at)">
              <mat-icon aria-hidden="true">delete</mat-icon>
            </button>
          </div>
          <mat-form-field appearance="outline" subscriptSizing="dynamic" class="part__field" [class.field-bad]="err?.owner">
            <mat-label>Para quem (parte {{ n }})</mat-label>
            <select matNativeControl [value]="part.characterId" [attr.aria-invalid]="err?.owner ? 'true' : null" (change)="edit(at, { characterId: $any($event.target).value })">
              <option value="" [selected]="part.characterId === ''">Sem dono ainda</option>
              @for (w of whoFor(part); track w.id) {
                <option [value]="w.id" [selected]="w.id === part.characterId" [disabled]="takenByAnother(w.id, at)">{{ w.name }}{{ takenByAnother(w.id, at) ? ' (já tem uma parte)' : '' }}</option>
              }
            </select>
            @if (err?.owner) {
              <mat-hint class="field-error" role="alert">{{ err?.owner }}</mat-hint>
            }
          </mat-form-field>
          @if (part.ownerUnavailable) {
            <p class="pf__note">Parte de um personagem que não está mais na mesa: ninguém a lê. Dê-a a outro.</p>
          }
          <mat-form-field appearance="outline" subscriptSizing="dynamic" class="part__field" [class.field-bad]="err?.text">
            <mat-label>O que ele lê (parte {{ n }})</mat-label>
            <textarea #text matInput rows="2" [value]="part.text" [attr.aria-invalid]="err?.text ? 'true' : null" (input)="edit(at, { text: $any($event.target).value })"></textarea>
            @if (err?.text) {
              <mat-hint class="field-error" role="alert">{{ err?.text }}</mat-hint>
            } @else {
              <mat-hint align="end">{{ length(part.text) }}&nbsp;de&nbsp;{{ partMax }}</mat-hint>
            }
          </mat-form-field>
        </div>
      }
      <button #add matButton type="button" class="pf__add" [disabled]="parts().length >= limit" disabledInteractive [class.mr-button--off]="parts().length >= limit" (click)="parts().length < limit && append()">
        <mat-icon aria-hidden="true">add</mat-icon>Adicionar parte
      </button>
      @if (parts().length >= limit) {
        <p class="pf__help">Máximo de {{ limit }} partes.</p>
      }
      @if (hasNoOwner()) {
        <p class="pf__note">Uma parte “Sem dono” ninguém lê. Dê-a a um personagem antes de mostrar o quebra-cabeça.</p>
      }
    </section>
  `,
  styleUrl: './parts-field.scss',
})
export class PartsField implements OnInit {
  private readonly roster = inject(RosterClient);
  private readonly injector = inject(Injector);
  protected readonly limit = PARTS_MAX;
  protected readonly partMax = PART_MAX;

  readonly campaignId = input.required<string>();
  readonly parts = input.required<readonly PartDraft[]>();
  /** What is wrong with each part, by index (the form's, then the server's). */
  readonly errors = input<Readonly<Record<number, { owner?: string; text?: string }>>>({});
  readonly partsChange = output<PartDraft[]>();

  protected readonly characters = signal<readonly Who[]>([]);
  protected readonly failed = signal(false);
  protected readonly hasNoOwner = computed(() => this.parts().some((p) => p.characterId === ''));

  private readonly texts = viewChildren<ElementRef<HTMLTextAreaElement>>('text');
  private readonly addButton = viewChild<string, ElementRef<HTMLElement>>('add', { read: ElementRef });

  ngOnInit(): void {
    this.roster.list(this.campaignId()).then(
      (all) =>
        this.characters.set(
          all.filter((c) => c.kind === CharacterKind.PLAYER && c.playerUserId !== '').map((c) => ({ id: c.id, name: c.name })),
        ),
      () => this.failed.set(true),
    );
  }

  /** The characters a part's select offers: the party, plus the part's own owner when they are no longer in it. */
  protected whoFor(part: PartDraft): readonly Who[] {
    const all = this.characters();
    return part.characterId !== '' && !all.some((w) => w.id === part.characterId) ? [...all, { id: part.characterId, name: 'Personagem que saiu da mesa' }] : all;
  }

  protected takenByAnother(id: string, index: number): boolean {
    return this.parts().some((p, i) => i !== index && p.characterId === id);
  }

  protected owner(part: PartDraft): string {
    if (part.characterId === '') {
      return 'Sem dono';
    }
    const who = this.whoFor(part).find((w) => w.id === part.characterId);
    return who ? `para ${who.name}` : '';
  }

  protected badge(part: PartDraft): string {
    const who = this.whoFor(part).find((w) => w.id === part.characterId);
    return who ? who.name.trim().charAt(0).toUpperCase() : '?';
  }

  protected length(text: string): number {
    return textLength(text.trim());
  }

  protected edit(index: number, partial: Partial<PartDraft>): void {
    this.partsChange.emit(this.parts().map((p, i) => (i === index ? { ...p, ...partial, ownerUnavailable: partial.characterId === undefined ? p.ownerUnavailable : false } : p)));
  }

  protected append(): void {
    this.partsChange.emit([...this.parts(), { characterId: '', text: '', ownerUnavailable: false }]);
    afterNextRender(() => this.texts().at(-1)?.nativeElement.focus(), { injector: this.injector });
  }

  protected remove(index: number): void {
    this.partsChange.emit(this.parts().filter((_, i) => i !== index));
    afterNextRender(() => this.addButton()?.nativeElement.focus(), { injector: this.injector });
  }
}
