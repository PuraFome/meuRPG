import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { RouterLink } from '@angular/router';

import { unreadWords } from './unread-notice';

export type SessionTab = 'scene' | 'group' | 'chat';

/** The badge stops counting here: "99+" is as much as a glance can use. */
const BADGE_MAX = 99;

/**
 * The phone's section nav, "Cena | Ficha | Conversa" (MR-051; the master's is "Cena | Grupo | Conversa", and "Combate | Grupo | Conversa" while a combat is on), pinned to the bottom of every live phone screen (the board's
 * "Cena · Ficha · Conversa"): "Cena" is the session page, "Ficha"
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
      <button type="button" class="tab" [attr.aria-current]="tab() === 'scene' ? 'true' : null" (click)="choose.emit('scene')">{{ sceneLabel() }}</button>
      @if (master()) {
        <button
          type="button"
          class="tab"
          [class.tab--wait]="groupCount() > 0"
          [attr.aria-current]="tab() === 'group' ? 'true' : null"
          [attr.aria-label]="groupName()"
          (click)="choose.emit('group')"
        >
          Grupo
          @if (groupCount() > 0) {
            <span class="tab__badge tab__badge--wait" aria-hidden="true">{{ groupCount() }}</span>
          }
        </button>
      } @else {
        <a class="tab" [routerLink]="sheetLink()">{{ sheetLabel() }}</a>
      }
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
  /** The first tab's word: "Cena", or "Combate" while the master's combat is on. */
  readonly sceneLabel = input('Cena');
  /** The second tab's word: "Ficha" for a player, "Grupo" for the master's combat (a link to the campaign page, where the party is). */
  readonly sheetLabel = input('Ficha');
  /** The master's nav outside a combat: "Grupo" (the party and the table's clock) is a tab of the board and takes the place of the link. */
  readonly master = input(false);
  /** What waits for the master in "Grupo" (a long cast): a count in amber. */
  readonly groupCount = input(0);
  readonly choose = output<SessionTab>();

  protected readonly badge = computed(() =>
    this.unread() > BADGE_MAX ? `${BADGE_MAX}+` : String(this.unread()),
  );
  protected readonly groupName = computed(() => {
    const n = this.groupCount();
    return n > 0
      ? `Grupo, ${n} ${n === 1 ? 'conjuração esperando' : 'conjurações esperando'}`
      : null;
  });
  protected readonly chatName = computed(() => {
    const n = this.unread();
    if (n === 0) {
      return null;
    }
    return `Conversa, ${unreadWords(n)}`;
  });
}
