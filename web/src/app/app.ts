import { Component, DestroyRef, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatIconModule } from '@angular/material/icon';
import { NavigationEnd, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { filter } from 'rxjs';

import { LivePill } from './shared/live-pill/live-pill';
import { LiveNotice } from './shell/live-notice/live-notice';
import { OpenSessionVm, OpenSessions, sessionForLiveLink } from './shell/live-notice/open-sessions';
import { UserMenu } from './shell/user-menu/user-menu';

/**
 * The app shell: the top bar, the page and the footer.
 *
 * On a phone the bar shows only the name and a menu button; the button
 * opens the same links and the account controls in a panel under the bar
 * (docs/design.md#espaço-forma-e-layout). The panel is plain markup, not a
 * MatMenu: the CDK overlay would land in the initial bundle, which this
 * component always sits in. From 768px up, the links are simply inline and
 * the button is hidden.
 *
 * While a session of one of the person's campaigns is open, the bar shows
 * the "Ao vivo" link to it (next to the menu button on a phone, before the
 * account on wider screens), and the session notice sits above the page
 * (RN-06; `shell/live-notice`). Both read `OpenSessions`, whose poll loads
 * the generated PlayService client lazily, so neither adds it to the
 * initial bundle.
 */
@Component({
  selector: 'app-root',
  imports: [
    RouterOutlet,
    RouterLink,
    RouterLinkActive,
    MatIconModule,
    LivePill,
    LiveNotice,
    UserMenu,
  ],
  templateUrl: './app.html',
  styleUrl: './app.scss',
  host: { '(document:keydown.escape)': 'menuOpen.set(false)' },
})
export class App {
  private readonly router = inject(Router);
  private readonly openSessions = inject(OpenSessions);

  protected readonly menuOpen = signal(false);
  /** The current URL: the notice and the "Ao vivo" link hide on some pages. */
  protected readonly url = signal(this.router.url);
  protected readonly liveSession = computed(() =>
    sessionForLiveLink(this.openSessions.sessions(), this.url()),
  );

  constructor() {
    // Following a link from the open panel closes it.
    this.router.events
      .pipe(
        filter((e) => e instanceof NavigationEnd),
        takeUntilDestroyed(inject(DestroyRef)),
      )
      .subscribe((e) => {
        this.menuOpen.set(false);
        this.url.set(e.urlAfterRedirects);
      });
  }

  /** The visible word comes first, then which session it opens. */
  protected liveLabel(session: OpenSessionVm): string {
    return `Ao vivo: sessão ${session.sessionNumber} de ${session.campaignName}`;
  }

  protected toggleMenu(): void {
    this.menuOpen.update((open) => !open);
  }
}
