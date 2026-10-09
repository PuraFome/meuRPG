import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  inject,
  signal,
} from '@angular/core';
import { toObservable } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { RouterLink } from '@angular/router';
import { filter, take } from 'rxjs';

import { AuthService, type AuthState } from '../../core/auth/auth.service';
import { submitLoginForm } from '../../core/auth/login-form';
import { claimFailure } from '../../core/characters/claim-errors';
import { ClaimsService } from '../../core/characters/claims.service';
import { Portrait } from '../../shared/portrait/portrait';

/** What the page shows: one state at a time. */
type State =
  | { readonly status: 'waiting-for-session' }
  | { readonly status: 'signed-out' }
  | { readonly status: 'loading' }
  | { readonly status: 'claiming' }
  | { readonly status: 'card'; readonly card: Card }
  | { readonly status: 'done'; readonly result: Result }
  | { readonly status: 'unusable' }
  | { readonly status: 'own-link' }
  | {
      readonly status: 'living';
      readonly campaignId: string;
      readonly characterId: string;
      readonly characterName: string;
    }
  | {
      readonly status: 'error';
      readonly message: string;
      readonly retry: 'session' | 'preview' | 'claim';
    };

interface Card {
  readonly characterName: string;
  readonly campaignName: string;
  readonly raceAndClass: string;
  readonly sentBy: string;
}

interface Result {
  readonly campaignId: string;
  readonly campaignName: string;
  readonly characterId: string;
  readonly characterName: string;
}

const TOKEN_PATTERN = /^#t=(.+)$/;

/**
 * "/claim#t=<token>" (public, MR-049): where a player takes a reserved character with the link the master sent.
 *
 * The secret sits after the `#`, which the browser never sends to a server. The constructor reads it and calls
 * `history.replaceState` at once, so it is not in the address bar, in `history` or in a screenshot; from then
 * on it lives in `token`, a private field, and goes only in the body of a POST (`PreviewClaim`,
 * `ClaimCharacter`, or the sign-in form's `intent_payload`). It is never a route parameter, a query parameter
 * or in Web Storage (docs/privacy.md).
 *
 * Signed out, the page is the same for every link: what a claim link is and "Entrar com Google". It never asks
 * the server about the link before the person has a session, so the page tells nothing about the campaign or
 * the character to whoever holds only a link. Signing in goes through `POST /auth/login` with
 * `intent=character_claim`, which only brings the person back here: signing in never claims. Back from the
 * provider the address has no secret, so the page asks the server with an empty token, which means the link
 * the person signed in with.
 *
 * Signed in, the page shows the public card and only "Assumir este personagem" claims. Every link that cannot
 * be used (invalid, expired, used, revoked, or a claim that lost a race) is the same page; the one specific
 * refusal is RN-03, a living character in this campaign already.
 */
@Component({
  selector: 'app-claim-page',
  imports: [MatButtonModule, MatIconModule, MatProgressSpinnerModule, Portrait, RouterLink],
  templateUrl: './claim-page.html',
  styleUrl: './claim-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ClaimPage {
  private readonly auth = inject(AuthService);
  private readonly claims = inject(ClaimsService);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  /** The link's secret, read once from `location.hash` and kept only in memory. `''` when the address had none:
   * the server then looks for the link the person signed in with. */
  private readonly token: string;

  protected readonly state = signal<State>({ status: 'waiting-for-session' });
  /** The card the person saw, for the sentence of RN-03 ("o link de Kai") and for "Tentar de novo". */
  private seen: Card | null = null;

  /** The signed-in person's name, for "Você entrou como Lia." (empty when they chose none). */
  protected readonly signedInAs = computed(() => {
    const s = this.auth.state();
    return s.status === 'signed-in' ? (s.user.displayName ?? '') : '';
  });

  constructor() {
    const match = TOKEN_PATTERN.exec(window.location.hash);
    let token = '';
    try {
      token = match ? decodeURIComponent(match[1]) : '';
    } catch {
      // A malformed percent-escape is no token at all.
    }
    this.token = token;
    // The secret must not stay in the address bar, in `history` or in anything that reads `location` later.
    // `replaceState`, not `pushState`: this is never an entry of its own.
    history.replaceState(null, '', window.location.pathname + window.location.search);

    toObservable(this.auth.state)
      .pipe(
        filter((s) => s.status !== 'unknown'),
        take(1),
      )
      .subscribe((s) => this.onSession(s));
  }

  private onSession(s: AuthState): void {
    if (s.status === 'signed-in') {
      void this.preview();
    } else if (s.status === 'signed-out') {
      this.show({ status: 'signed-out' });
    } else {
      this.show({
        status: 'error',
        message: 'Não foi possível confirmar sua sessão agora. Tente de novo em instantes.',
        retry: 'session',
      });
    }
  }

  /** Puts a state on screen and moves the focus to the page's main button (or its title). */
  private show(state: State): void {
    this.state.set(state);
    afterNextRender(
      () => {
        const target =
          this.host.nativeElement.querySelector<HTMLElement>('[data-initial-focus]') ??
          this.host.nativeElement.querySelector<HTMLElement>('h1');
        target?.focus({ preventScroll: true });
      },
      { injector: this.injector },
    );
  }

  private async preview(): Promise<void> {
    this.state.set({ status: 'loading' });
    try {
      const res = await this.claims.preview(this.token);
      if (res.ownLink) {
        this.show({ status: 'own-link' });
        return;
      }
      const card = res.card;
      if (!card) {
        this.show({ status: 'unusable' });
        return;
      }
      this.seen = {
        characterName: card.characterName,
        campaignName: card.campaignName,
        raceAndClass: [card.raceNamePt, card.classSummary].filter((p) => p !== '').join(' · '),
        sentBy: card.sentByDisplayName,
      };
      this.show({ status: 'card', card: this.seen });
    } catch (err) {
      this.fail(err, 'preview');
    }
  }

  /** "Assumir este personagem": the only thing on this page that claims. */
  protected async claim(): Promise<void> {
    if (this.state().status !== 'card') {
      return;
    }
    this.state.set({ status: 'claiming' });
    try {
      const res = await this.claims.claim(this.token);
      this.show({
        status: 'done',
        result: {
          campaignId: res.campaignId,
          campaignName: res.campaignName,
          characterId: res.characterId,
          characterName: res.characterName,
        },
      });
    } catch (err) {
      this.fail(err, 'claim');
    }
  }

  private fail(err: unknown, retry: 'preview' | 'claim'): void {
    const failure = claimFailure(err);
    switch (failure.kind) {
      case 'unusable':
        this.show({ status: 'unusable' });
        break;
      case 'own-link':
        this.show({ status: 'own-link' });
        break;
      case 'living':
        this.show({
          status: 'living',
          campaignId: failure.campaignId,
          characterId: failure.characterId,
          characterName: failure.characterName,
        });
        break;
      case 'signed-out':
        void this.auth.refresh().then(() => this.onSession(this.auth.state()));
        break;
      case 'transient':
        this.show({ status: 'error', message: failure.message, retry });
        break;
    }
  }

  /** "Tentar de novo": the address has no secret any more, so a reload could not do it; the one in memory can. */
  protected async retry(): Promise<void> {
    const s = this.state();
    if (s.status !== 'error') {
      return;
    }
    if (s.retry === 'claim' && this.seen) {
      this.state.set({ status: 'card', card: this.seen });
      await this.claim();
    } else if (s.retry === 'session') {
      this.state.set({ status: 'waiting-for-session' });
      await this.auth.refresh();
      this.onSession(this.auth.state());
    } else {
      await this.preview();
    }
  }

  /** "Entrar com Google": the sign-in carries the link along, only to come back to this page. It never claims. */
  protected signIn(): void {
    submitLoginForm(this.loginFields());
  }

  /** "Não é você? Entrar com outra conta": ends this session and asks the provider to offer the choice of account. */
  protected async signInAsAnother(): Promise<void> {
    await this.auth.endSessionHere();
    submitLoginForm({ ...this.loginFields(), prompt: 'select_account' });
  }

  private loginFields(): Record<string, string> {
    return this.token === ''
      ? { return_to: '/claim' }
      : { return_to: '/claim', intent: 'character_claim', intent_payload: this.token };
  }

  /** The name on the card, for the words of RN-03's page. */
  protected readonly cardName = () => this.seen?.characterName ?? '';
}
