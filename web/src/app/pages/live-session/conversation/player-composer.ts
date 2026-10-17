import { ChangeDetectionStrategy, Component, computed, input, signal } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import type { ConversationState } from '../../../core/conversation/conversation-state';
import { Segmented, type Segment } from '../../../shared/segmented/segmented';
import { ComposerField } from './composer-field';

type As = 'character' | 'out-of-character';
type To = 'all' | 'master';

/**
 * The player's composer (MR-051): "Como <personagem>" or "Fora do personagem", to everyone or
 * only to the master, a message box and "Enviar" (an icon button of 48 px on a phone). The line
 * is shown at once by the state; a refusal (empty, too long) is told here and keeps the text.
 */
@Component({
  selector: 'app-player-composer',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [ComposerField, MatIconModule, Segmented],
  template: `
    <form class="composer" (submit)="submit($event)">
      <div class="composer__choices">
        <app-segmented label="Como você fala" [segments]="asSegments()" [value]="speech()" (choose)="choice.set($event)" />
        <app-segmented label="Para" [segments]="toSegments" [value]="to()" (choose)="to.set($event)" />
      </div>
      <div class="composer__row">
        <app-composer-field
          class="composer__field"
          [(value)]="text"
          [error]="error()"
          [placeholder]="speech() === 'character' ? 'Escreva como ' + characterName() : 'Escreva fora do personagem'"
          (changed)="error.set('')"
          (sent)="submit()"
        />
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
  protected readonly to = signal<To>('all');
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
  protected readonly toSegments: readonly Segment<To>[] = [
    { value: 'all', label: 'Todos' },
    { value: 'master', label: 'Só o mestre' },
  ];

  protected submit(event?: Event): void {
    event?.preventDefault();
    const problem = this.state().send({
      role: 'player',
      text: this.text(),
      as: this.speech(),
      characterId: this.characterId(),
      onlyMaster: this.to() === 'master',
    });
    this.error.set(problem ?? '');
    if (!problem) {
      this.text.set('');
    }
  }
}
