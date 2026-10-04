import {
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  inject,
  input,
  output,
  viewChild,
} from '@angular/core';
import { ReactiveFormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';

import type { NoteScene } from '../../../gen/meurpg/notes/v1/notes_pb';
import type { NoteEditing } from '../../core/notes/note-editing';
import { noteCounter } from '../../core/notes/notes-view';
import { FictionNotice } from '../fiction-notice/fiction-notice';
import { NotesSelect, type SelectOption } from './notes-select';

/**
 * The fields of "Nova anotação" and "Editar anotação" (E8-06, E8-07), the same
 * in the notes sheet and in the character sheet's panel: the text (6 lines to
 * start, it grows; "Só você lê." and the counter "45 de 2.000" under it), the
 * scene tag ("Cena (opcional)": "Sem cena" and the scenes the group already
 * discovered), the shared "É ficção" notice, what went wrong, and, when
 * editing, "Apagar anotação" as text at the end, which asks in place first
 * ("Apagar esta anotação? Não dá para desfazer.") with "Voltar" focused. The
 * buttons that save and cancel are the parent's: the sheet keeps them in its
 * fixed footer.
 *
 * The field stops accepting at the 2.000th character, so nothing is lost
 * silently: at the limit the counter turns red and a message says so, with an
 * icon and words. The focus starts on the text.
 */
@Component({
  selector: 'app-note-fields',
  imports: [
    FictionNotice,
    MatButtonModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    NotesSelect,
    ReactiveFormsModule,
  ],
  templateUrl: './note-fields.html',
  styleUrl: './note-fields.scss',
})
export class NoteFields {
  private readonly injector = inject(Injector);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  readonly editing = input.required<NoteEditing>();
  readonly scenes = input.required<readonly NoteScene[]>();
  /** The note was deleted (the parent shows the list again). */
  readonly removed = output<void>();

  protected readonly options = computed<readonly SelectOption[]>(() => [
    { value: '', label: 'Sem cena' },
    ...this.scenes().map((s) => ({ value: s.id, label: s.name })),
  ]);
  protected readonly counter = computed(() => noteCounter(this.editing().length(), this.editing().max));
  protected readonly atLimit = computed(() => this.editing().length() >= this.editing().max);

  private readonly field = viewChild.required<ElementRef<HTMLTextAreaElement>>('field');
  private readonly back = viewChild('back', { read: ElementRef<HTMLButtonElement> });

  constructor() {
    afterNextRender(
      () => {
        // The whole form comes into view, then the cursor goes to the text without scrolling again.
        this.host.nativeElement.scrollIntoView({ block: 'nearest' });
        this.field().nativeElement.focus({ preventScroll: true });
      },
      { injector: this.injector },
    );
  }

  /** "Apagar anotação": the question opens in place, on its safe answer. */
  protected askDelete(): void {
    this.editing().askDelete();
    afterNextRender(
      () => {
        const back = this.back()?.nativeElement;
        back?.closest('.nf__ask')?.scrollIntoView({ block: 'nearest' });
        back?.focus({ preventScroll: true });
      },
      { injector: this.injector },
    );
  }

  /** The error box has the words; an input the person is still typing in is not told again. */
  protected async confirmDelete(): Promise<void> {
    if (await this.editing().remove()) {
      this.removed.emit();
    }
  }

  /** Focus back on the text (after a message about it). */
  focusText(): void {
    this.field().nativeElement.focus();
  }
}
