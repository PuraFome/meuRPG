import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/** "3 mensagens novas", "1 mensagem nova": the news in words, for the notice and the tab's name. */
export function unreadWords(n: number): string {
  return `${n} ${n === 1 ? 'mensagem nova' : 'mensagens novas'}`;
}

/**
 * The phone's sign of new lines, at the top of the session page (MR-051): the "Conversa" tab closes
 * the page, under the board, so its badge is out of sight until the reader scrolls. This is one
 * quiet row, drawn only while there is news and the chat is closed, whose whole width opens the
 * chat. The space between the two spans is kept (`&ngsp;`): without it the name read from the content is
 * "mensagem novaAbrir". It is not a live region: the open panel already tells each line politely, once.
 */
@Component({
  selector: 'app-unread-notice',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    <div class="mr-notice mr-notice--neutral news">
      <button type="button" class="news__open" (click)="open.emit()">
        <mat-icon aria-hidden="true">chat</mat-icon>
        <span class="news__text">Conversa: {{ words() }}</span>&ngsp;<span class="news__go">Abrir</span>
      </button>
    </div>
  `,
  styleUrl: './unread-notice.scss',
})
export class UnreadNotice {
  readonly unread = input.required<number>();
  readonly open = output();

  protected words(): string {
    return unreadWords(this.unread());
  }
}
