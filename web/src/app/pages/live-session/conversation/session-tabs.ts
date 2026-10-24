import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { RouterLink } from '@angular/router';

export type SessionTab = 'scene' | 'chat';

/** The badge stops counting here: "99+" is as much as a glance can use. */
const BADGE_MAX = 99;

/**
 * The phone's section nav, "Cena | Ficha | Conversa" (MR-051): "Cena" is the session page, "Ficha"
 * goes to the character (the master's to the campaign, where the party is), and "Conversa" opens
 * the full-screen conversation. The unread count sits on "Conversa" in the accent colour and is
 * spoken in words in the button's name ("Conversa, 3 mensagens novas").
 */
@Component({
  selector: 'app-session-tabs',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [RouterLink],
  template: `
    <nav class="tabs" aria-label="Seções da sessão">
      <button type="button" class="tab" [attr.aria-current]="tab() === 'scene' ? 'true' : null" (click)="choose.emit('scene')">Cena</button>
      <a class="tab" [routerLink]="sheetLink()">Ficha</a>
      <button
        type="button"
        class="tab"
        [class.tab--news]="unread() > 0"
        [attr.aria-current]="tab() === 'chat' ? 'true' : null"
        [attr.aria-label]="chatName()"
        (click)="choose.emit('chat')"
      >
        Conversa
        @if (unread() > 0) {
          <span class="tab__badge" aria-hidden="true">{{ badge() }}</span>
        }
      </button>
    </nav>
  `,
  styleUrl: './session-tabs.scss',
})
export class SessionTabs {
  readonly tab = input.required<SessionTab>();
  readonly unread = input(0);
  readonly sheetLink = input.required<readonly string[]>();
  readonly choose = output<SessionTab>();

  protected readonly badge = computed(() =>
    this.unread() > BADGE_MAX ? `${BADGE_MAX}+` : String(this.unread()),
  );
  protected readonly chatName = computed(() => {
    const n = this.unread();
    if (n === 0) {
      return null;
    }
    return `Conversa, ${n} ${n === 1 ? 'mensagem nova' : 'mensagens novas'}`;
  });
}
