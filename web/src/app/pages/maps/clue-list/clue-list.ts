import {
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  inject,
  input,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { SceneClue } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { tight } from '../../../core/format/text';
import { sceneClueErrorMessage } from '../../../core/maps/map-errors';
import { MapsClient } from '../../../core/maps/maps-client';
import { ActionKey } from '../../../core/connect/idempotency';
import {
  CLUE_LIMIT,
  type CluePlayer,
  clueAudience,
  moreClues,
} from '../../../core/maps/scene-clues';
import { ClueForm } from './clue-form';

/** Past this many rows the list folds, with "Mais 28 pistas na lista." and a button. */
const FOLD_AFTER = 8;

type Control = 'text' | 'up' | 'down' | 'remove';

/**
 * "Pistas" in the point panel of a SCENE point (E8-04, MR-029): what the
 * players may find in the scene, in the master's order, each with who has it
 * already ("Todos", "Só Brisa", "Ninguém ainda"; revealing happens in the open
 * scene, so here the status is only read). Like the scene actions, every change
 * saves at once (add, edit, ↑, ↓, remove) and the panel's "Salvar ponto" never
 * carries them. One write at a time: while one is in flight the other buttons
 * answer nothing (`aria-disabled`, not `disabled`, so focus stays where it was).
 *
 * Removing asks in place ("Remover a pista 2? Quem já recebeu continua com
 * ela."), with "Voltar" focused first. Tapping a clue's text opens it for
 * editing in place.
 *
 * Focus: after ↑ or ↓ it stays on the same button of the moved row; after a
 * removal it goes to the next row's "Remover", or to "Adicionar pista" when
 * there is none; a form that closes hands focus back to where it opened.
 */
@Component({
  selector: 'app-clue-list',
  imports: [ClueForm, MatButtonModule, MatIconModule],
  templateUrl: './clue-list.html',
  styleUrl: './clue-list.scss',
})
export class ClueList {
  private readonly api = inject(MapsClient);
  private readonly addKey = new ActionKey();
  private readonly injector = inject(Injector);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  readonly campaignId = input.required<string>();
  readonly mapId = input.required<string>();
  readonly pointId = input.required<string>();
  readonly clues = input.required<readonly SceneClue[]>();
  /** The campaign's player characters, to tell "Todos" from "Só Brisa". */
  readonly players = input<readonly CluePlayer[]>([]);
  /** The point's clues as the server has them now (after each write). */
  readonly cluesChange = output<readonly SceneClue[]>();

  protected readonly limit = CLUE_LIMIT;
  protected readonly count = computed(() => tight(`${this.clues().length} de ${CLUE_LIMIT}`));
  protected readonly adding = signal(false);
  protected readonly editingId = signal<string | null>(null);
  protected readonly confirmingId = signal<string | null>(null);
  protected readonly expanded = signal(false);
  protected readonly busy = signal(false);
  /** A refusal of the add or edit form, shown inside it. */
  protected readonly formError = signal('');
  /** A refusal of a move or a removal. */
  protected readonly error = signal('');
  /** What a screen reader hears after a change. */
  protected readonly status = signal('');
  protected readonly full = computed(() => this.clues().length >= CLUE_LIMIT);
  protected readonly shown = computed(() => {
    const all = this.clues();
    return all.length > FOLD_AFTER && !this.expanded() ? all.slice(0, FOLD_AFTER) : all;
  });
  protected readonly hiddenCount = computed(() => this.clues().length - this.shown().length);
  protected readonly more = computed(() => tight(moreClues(this.hiddenCount())));
  protected readonly audience = (clue: SceneClue) => clueAudience(clue, this.players());
  protected readonly icon = (kind: 'all' | 'some' | 'none') =>
    kind === 'all' ? 'groups' : kind === 'some' ? 'person' : 'visibility_off';
  protected readonly editing = computed(() => {
    const id = this.editingId();
    const index = this.clues().findIndex((c) => c.id === id);
    return index < 0 ? null : { text: this.clues()[index].text, number: index + 1 };
  });

  private readonly addButton = viewChild('addButton', { read: ElementRef<HTMLButtonElement> });

  protected openForm(): void {
    if (this.full()) {
      return;
    }
    this.formError.set('');
    this.editingId.set(null);
    this.confirmingId.set(null);
    this.adding.set(true);
  }

  protected closeForm(): void {
    this.adding.set(false);
    this.formError.set('');
    this.focusAdd();
  }

  protected startEdit(clue: SceneClue): void {
    this.formError.set('');
    this.adding.set(false);
    this.confirmingId.set(null);
    this.editingId.set(clue.id);
  }

  protected stopEdit(): void {
    const id = this.editingId();
    this.editingId.set(null);
    this.formError.set('');
    if (id) {
      this.focusRow(id, 'text');
    }
  }

  protected askRemove(clue: SceneClue): void {
    if (this.busy()) {
      return;
    }
    this.adding.set(false);
    this.editingId.set(null);
    this.error.set('');
    this.confirmingId.set(clue.id);
    this.focusConfirm(clue.id, 'back');
  }

  protected cancelRemove(clue: SceneClue): void {
    this.confirmingId.set(null);
    this.focusRow(clue.id, 'remove');
  }

  protected async add(text: string): Promise<void> {
    if (this.busy()) {
      return;
    }
    this.busy.set(true);
    this.formError.set('');
    try {
      const clues = await this.api.addSceneClue(
        this.campaignId(),
        this.mapId(),
        this.pointId(),
        text,
        this.addKey.keyFor([this.pointId(), text]),
      );
      this.addKey.renew();
      this.cluesChange.emit(clues);
      this.adding.set(false);
      this.status.set(`Pista adicionada. ${clues.length} de ${CLUE_LIMIT}.`);
      this.focusAdd();
    } catch (err) {
      this.formError.set(sceneClueErrorMessage(err, 'adicionar a pista'));
    } finally {
      this.busy.set(false);
    }
  }

  protected async save(clue: SceneClue, text: string): Promise<void> {
    if (this.busy()) {
      return;
    }
    this.busy.set(true);
    this.formError.set('');
    try {
      const clues = await this.api.updateSceneClue(
        this.campaignId(),
        this.mapId(),
        this.pointId(),
        clue.id,
        text,
      );
      this.cluesChange.emit(clues);
      this.editingId.set(null);
      this.status.set('Pista salva.');
      this.focusRow(clue.id, 'text');
    } catch (err) {
      this.formError.set(sceneClueErrorMessage(err, 'salvar a pista'));
    } finally {
      this.busy.set(false);
    }
  }

  protected async move(clue: SceneClue, direction: 'up' | 'down'): Promise<void> {
    const all = this.clues();
    const index = all.findIndex((c) => c.id === clue.id);
    const edge = direction === 'up' ? index === 0 : index === all.length - 1;
    if (this.busy() || edge) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      const clues = await this.api.moveSceneClue(
        this.campaignId(),
        this.mapId(),
        this.pointId(),
        clue.id,
        direction,
      );
      this.cluesChange.emit(clues);
      const place = clues.findIndex((c) => c.id === clue.id) + 1;
      this.status.set(
        `Pista ${index + 1} ${direction === 'up' ? 'subiu' : 'desceu'} para a posição ${place}.`,
      );
      // The moved row keeps the same button, as soon as it is drawn.
      this.focusRow(clue.id, direction);
    } catch (err) {
      this.error.set(sceneClueErrorMessage(err, 'mover a pista'));
    } finally {
      this.busy.set(false);
    }
  }

  protected async remove(clue: SceneClue): Promise<void> {
    if (this.busy()) {
      return;
    }
    const index = this.clues().findIndex((c) => c.id === clue.id);
    this.busy.set(true);
    this.error.set('');
    try {
      const clues = await this.api.removeSceneClue(
        this.campaignId(),
        this.mapId(),
        this.pointId(),
        clue.id,
      );
      this.cluesChange.emit(clues);
      this.confirmingId.set(null);
      this.status.set(`Pista ${index + 1} removida.`);
      const next = clues[index];
      if (next) {
        this.focusRow(next.id, 'remove');
      } else {
        this.focusAdd();
      }
    } catch (err) {
      this.confirmingId.set(null);
      this.error.set(sceneClueErrorMessage(err, 'remover a pista'));
    } finally {
      this.busy.set(false);
    }
  }

  private focusRow(clueId: string, control: Control): void {
    afterNextRender(
      () =>
        this.host.nativeElement
          .querySelector<HTMLElement>(`[data-clue="${clueId}"][data-control="${control}"]`)
          ?.focus(),
      { injector: this.injector },
    );
  }

  /** The question's safe button first; the row scrolls whole into view under the sticky bar. */
  private focusConfirm(clueId: string, control: 'back' | 'remove'): void {
    afterNextRender(
      () => {
        const target = this.host.nativeElement.querySelector<HTMLElement>(
          `[data-confirm="${clueId}"][data-control="${control}"]`,
        );
        target?.closest<HTMLElement>('.cl__ask')?.scrollIntoView({ block: 'nearest' });
        target?.focus({ preventScroll: true });
      },
      { injector: this.injector },
    );
  }

  private focusAdd(): void {
    afterNextRender(() => this.addButton()?.nativeElement.focus(), { injector: this.injector });
  }
}
