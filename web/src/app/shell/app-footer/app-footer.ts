import { ChangeDetectionStrategy, Component } from '@angular/core';
import { RouterLink, RouterLinkActive } from '@angular/router';

/**
 * The footer of every page (docs/design.md#footer): the d20 in one grey, the year, and the three
 * pages that are always reachable, signed in or not. Quiet on purpose: 14px, the muted ink, below a
 * thin rule. It sits in the flow after the page, never fixed, so it covers nothing; it is left out
 * of the printed page.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-footer',
  imports: [RouterLink, RouterLinkActive],
  templateUrl: './app-footer.html',
  styleUrl: './app-footer.scss',
})
export class AppFooter {
  protected readonly year = new Date().getFullYear();
  protected readonly links = [
    { path: '/terms', label: 'Termos de uso' },
    { path: '/privacy', label: 'Privacidade' },
    { path: '/credits', label: 'Créditos' },
  ] as const;
}
