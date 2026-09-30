import { Component, DestroyRef, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatIconModule } from '@angular/material/icon';
import { NavigationEnd, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { filter } from 'rxjs';

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
 */
@Component({
  selector: 'app-root',
  imports: [RouterOutlet, RouterLink, RouterLinkActive, MatIconModule, UserMenu],
  templateUrl: './app.html',
  styleUrl: './app.scss',
  host: { '(document:keydown.escape)': 'menuOpen.set(false)' },
})
export class App {
  protected readonly menuOpen = signal(false);

  constructor() {
    // Following a link from the open panel closes it.
    inject(Router)
      .events.pipe(
        filter((e) => e instanceof NavigationEnd),
        takeUntilDestroyed(inject(DestroyRef)),
      )
      .subscribe(() => this.menuOpen.set(false));
  }

  protected toggleMenu(): void {
    this.menuOpen.update((open) => !open);
  }
}
