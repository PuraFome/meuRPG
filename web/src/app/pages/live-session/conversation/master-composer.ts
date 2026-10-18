import {
  ChangeDetectionStrategy,
  Component,
  Injector,
  computed,
  inject,
  input,
  signal,
} from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatSelectModule } from '@angular/material/select';

import type { ConversationState } from '../../../core/conversation/conversation-state';
import { type PlayerDirectory, readersSummary } from '../../../core/conversation/message-view';
import { openImagePicker } from '../../../shared/gallery-picker/image-picker-dialog/image-picker-dialog';
import { PHONE_QUERY, mediaQuery } from '../../../shared/map-view/media-query';
import { openSheet } from '../../../shared/sheet/sheet-host';
import { AudienceChooser, type AudienceData } from './audience-chooser';
import { ComposerField } from './composer-field';
import type { ChooserPlayer, Speaker } from './conversation-roster';

let nextId = 0;

interface PickedImage {
  readonly id: string;
  readonly name: string;
}

/**
 * The master's composer (MR-051): who speaks ("Falar como": the narrator, then the NPCs, those on
 * stage first), who reads ("Para": the whole table, or the people chosen in a sheet), a picture
 * from the gallery picker the app already has, the message and "Enviar". While the master types
 * the state tells the players, at most once in two seconds.
 */
@Component({
  selector: 'app-master-composer',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [ComposerField, MatIconModule, MatSelectModule],
  template: `
    <form class="composer" (submit)="submit($event)">
      <div class="composer__pair">
        <div class="composer__cell">
          <span class="composer__label" [id]="id + '-as'">Falar como</span>
          <mat-select
            class="composer__select"
            panelClass="mr-speaker-panel"
            disableOptionCentering
            hideSingleSelectionIndicator
            [aria-labelledby]="id + '-as'"
            [value]="speaker()"
            (selectionChange)="speaker.set($event.value)"
          >
            <mat-select-trigger>{{ speakerName() }}</mat-select-trigger>
            @for (s of speakers(); track s.value) {
              <mat-option [value]="s.value" class="speaker">{{ s.label }}@if (s.onStage) {<span class="speaker__stage"> · na cena</span>}</mat-option>
            }
          </mat-select>
        </div>
        <div class="composer__cell">
          <label class="composer__label" [for]="id + '-to'">Para</label>
          <button type="button" class="composer__to" [id]="id + '-to'" aria-haspopup="dialog" (click)="chooseReaders()">
            <span class="composer__to-text">{{ readers() }}</span>
            <mat-icon aria-hidden="true">group</mat-icon>
          </button>
        </div>
      </div>
      <app-composer-field
        [(value)]="text"
        [rows]="3"
        [dashed]="readerIds().length > 0"
        [error]="error()"
        [placeholder]="placeholder()"
        (changed)="typed()"
        (sent)="submit()"
      />
      @if (image(); as picked) {
        <p class="composer__chip">
          <mat-icon aria-hidden="true">image</mat-icon>
          <span class="composer__chip-name">{{ picked.name }}</span>
          <button type="button" class="composer__chip-remove" [attr.aria-label]="'Tirar a imagem ' + picked.name" (click)="image.set(null)">
            <mat-icon aria-hidden="true">close</mat-icon>
          </button>
        </p>
      }
      <div class="composer__actions">
        <button type="button" class="composer__image" (click)="pickImage()">
          <mat-icon aria-hidden="true">image</mat-icon>
          <span class="composer__image-text">Imagem da galeria</span>
        </button>
        <button type="submit" class="composer__send">
          <mat-icon aria-hidden="true">send</mat-icon>
          <span class="composer__send-text">Enviar</span>
        </button>
      </div>
    </form>
  `,
  styleUrl: './master-composer.scss',
})
export class MasterComposer {
  readonly state = input.required<ConversationState>();
  readonly campaignId = input.required<string>();
  readonly speakers = input.required<readonly Speaker[]>();
  readonly players = input.required<readonly ChooserPlayer[]>();
  readonly directory = input.required<PlayerDirectory>();

  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);
  private readonly injector = inject(Injector);
  private readonly phone = mediaQuery(PHONE_QUERY);

  protected readonly id = `mc-${nextId++}`;
  /** The NPC's character id; empty is the narrator. */
  protected readonly speaker = signal('');
  protected readonly text = signal('');
  /** Empty: the whole table. */
  protected readonly readerIds = signal<readonly string[]>([]);
  protected readonly image = signal<PickedImage | null>(null);
  protected readonly error = signal('');

  protected readonly readers = computed(() => readersSummary(this.readerIds(), this.directory()));
  protected readonly speakerName = computed(
    () => this.speakers().find((s) => s.value === this.speaker())?.label ?? 'Narrador',
  );
  protected readonly placeholder = computed(() => {
    const who = this.speaker() === '' ? 'Narre a cena' : `Fale como ${this.speakerName()}`;
    return this.readerIds().length > 0 ? `${who}, para ${this.readers()}` : who;
  });

  protected typed(): void {
    this.error.set('');
    this.state().signalTyping();
  }

  /** "Para": the chooser opens with the readers now chosen; "Cancelar" answers nothing and keeps them. */
  protected chooseReaders(): void {
    const data: AudienceData = { players: this.players(), selected: this.readerIds() };
    openSheet<AudienceChooser, AudienceData, readonly string[]>(
      this.dialog,
      this.bottomSheet,
      AudienceChooser,
      { data, ariaLabel: 'Quem lê esta mensagem?', labelledBy: 'audience-title' },
    ).subscribe((chosen) => {
      if (chosen) {
        this.readerIds.set(chosen);
      }
    });
  }

  /** "Imagem da galeria": the app's own picker; the image waits as a chip until the line is sent. */
  protected pickImage(): void {
    openImagePicker(
      this.dialog,
      {
        campaignId: this.campaignId(),
        title: 'Imagem da galeria',
        lead: 'A imagem aparece acima do texto da mensagem, para quem a lê.',
        confirmLabel: 'Usar esta imagem',
        current: null,
        currentTag: '',
        currentNote: '',
        note: () => 'Quem não lê a mensagem não vê a imagem.',
        hiddenMapImages: new Map(),
        emptyError: 'Escolha uma imagem.',
        wholeMapConfirmLabel: 'Usar mesmo assim',
        submit: (image) => {
          this.image.set({ id: image.id, name: image.name });
          return Promise.resolve();
        },
        errorMessage: () => 'Não deu para usar essa imagem. Tente de novo.',
      },
      this.injector,
      this.phone(),
    );
  }

  protected submit(event?: Event): void {
    event?.preventDefault();
    const npcId = this.speaker();
    const problem = this.state().send({
      role: 'master',
      text: this.text(),
      speaker: npcId === '' ? { kind: 'narrator' } : { kind: 'npc', npcId },
      readerUserIds: this.readerIds(),
      imageId: this.image()?.id,
    });
    this.error.set(problem ?? '');
    if (!problem) {
      // The voice and the readers stay: a scene goes on in the same voice and for the same people.
      this.text.set('');
      this.image.set(null);
    }
  }
}
