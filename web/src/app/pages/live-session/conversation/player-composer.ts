import { ChangeDetectionStrategy, Component, computed, input, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { ConversationState } from '../../../core/conversation/conversation-state';
import { Segmented, type Segment } from '../../../shared/segmented/segmented';
import { ComposerField } from './composer-field';

type As = 'character' | 'out-of-character';

/**
 * The player's composer (MR-051): one compact switch "Como <personagem>" | "Fora do personagem", a
 * message box, a toggle "Só o mestre lê" (the default is the whole table) and "Enviar" (an icon button of 48 px on a phone). The line
 * is shown at once by the state; a refusal (empty, too long) is told here and keeps the text.
 */
@Component({
  selector: 'app-player-composer',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [ComposerField, MatIconModule, Segmented],
  template: `
    <form class="composer" (submit)="submit($event)">
      <app-segmented appearance="track" label="Como você fala" [segments]="asSegments()" [value]="speech()" (choose)="choice.set($event)" />
      <div class="composer__row">
        <app-composer-field
          class="composer__field"
          [(value)]="text"
          [error]="error()"
          [placeholder]="placeholder()"
          (changed)="error.set('')"
          (sent)="submit()"
        />
        <button
          type="button"
          class="composer__private"
          title="Só o mestre lê"
          [attr.aria-pressed]="onlyMaster()"
          (click)="onlyMaster.update((on) => !on)"
        >
          <mat-icon aria-hidden="true">{{ onlyMaster() ? 'lock' : 'lock_open' }}</mat-icon>
          <span class="composer__private-text">Só o mestre lê</span>
        </button>
        <button type="submit" class="composer__send">
          <mat-icon aria-hidden="true">send</mat-icon>
          <span class="composer__send-text">Enviar</span>
        </button>
      </div>
    </form>
  `,
  styleUrl: './player-composer.scss',
})
export class PlayerComposer {
  readonly state = input.required<ConversationState>();
  /** The player's living character; empty when they have none. */
  readonly characterId = input.required<string>();
  readonly characterName = input.required<string>();

  protected readonly choice = signal<As>('character');
  /** "Para: só o mestre" (the default is the whole table). */
  protected readonly onlyMaster = signal(false);
  protected readonly text = signal('');
  protected readonly error = signal('');

  /** Without a living character the only way to speak is out of it. */
  protected readonly speech = computed<As>(() =>
    this.characterId() === '' ? 'out-of-character' : this.choice(),
  );
  protected readonly asSegments = computed<readonly Segment<As>[]>(() => [
    {
      value: 'character',
      label: `Como ${this.characterName()}`,
      disabled: this.characterId() === '',
    },
    { value: 'out-of-character', label: 'Fora do personagem' },
  ]);
  protected readonly placeholder = computed(() => {
    const who =
      this.speech() === 'character' ? `como ${this.characterName()}` : 'fora do personagem';
    return this.onlyMaster() ? `Escreva ${who}, só o mestre lê` : `Escreva ${who}`;
  });

  protected submit(event?: Event): void {
    event?.preventDefault();
    const problem = this.state().send({
      role: 'player',
      text: this.text(),
      as: this.speech(),
      characterId: this.characterId(),
      onlyMaster: this.onlyMaster(),
    });
    this.error.set(problem ?? '');
    if (!problem) {
      this.text.set('');
    }
  }
}
