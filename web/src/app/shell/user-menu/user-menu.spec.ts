import { Injectable, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';

import { AuthState, AuthService } from '../../core/auth/auth.service';
import { UserMenu } from './user-menu';

/** A stand-in for AuthService whose state this spec fully controls. */
@Injectable()
class FakeAuthService {
  private readonly stateSignal = signal<AuthState>({ status: 'unknown' });
  readonly state = this.stateSignal.asReadonly();
  readonly signIn = vi.fn();
  readonly signOut = vi.fn().mockResolvedValue(undefined);

  set(state: AuthState): void {
    this.stateSignal.set(state);
  }
}

describe('UserMenu', () => {
  let auth: FakeAuthService;

  beforeEach(() => {
    TestBed.configureTestingModule({
      imports: [UserMenu],
      providers: [provideRouter([]), { provide: AuthService, useClass: FakeAuthService }],
    });
    auth = TestBed.inject(AuthService) as unknown as FakeAuthService;
  });

  function render(): HTMLElement {
    const fixture = TestBed.createComponent(UserMenu);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('renders nothing while the session state is unknown (no flash of "Entrar")', () => {
    const el = render();
    expect(el.textContent?.trim()).toBe('');
  });

  it('shows an "Entrar" button when signed out, that navigates to sign-in with the current path', () => {
    auth.set({ status: 'signed-out' });
    // Stand in for actually navigating there — this component only reads
    // Router.url, it never triggers a route change.
    vi.spyOn(TestBed.inject(Router), 'url', 'get').mockReturnValue('/campaigns');

    const el = render();
    const button = el.querySelector('button');
    expect(button?.textContent).toContain('Entrar');

    button?.dispatchEvent(new Event('click', { bubbles: true }));
    expect(auth.signIn).toHaveBeenCalledWith('/campaigns');
  });

  it('shows "Minha conta" (never an e-mail) when signed in, with a Sair button', () => {
    auth.set({
      status: 'signed-in',
      user: { id: 'user-1', displayName: null },
      sessionExpiresAt: null,
    });

    const el = render();
    expect(el.textContent).toContain('Minha conta');

    const signOutButton = Array.from(el.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('Sair'),
    );
    expect(signOutButton).toBeTruthy();

    signOutButton?.dispatchEvent(new Event('click', { bubbles: true }));
    expect(auth.signOut).toHaveBeenCalled();
  });

  it('links the account label to "Meu perfil" when signed in', () => {
    auth.set({
      status: 'signed-in',
      user: { id: 'user-1', displayName: null },
      sessionExpiresAt: null,
    });

    const el = render();
    expect(el.querySelector('a[href="/profile"]')).toBeTruthy();
  });

  it('uses the display name instead of "Minha conta" once one is available', () => {
    auth.set({
      status: 'signed-in',
      user: { id: 'user-1', displayName: 'Vinicius' },
      sessionExpiresAt: null,
    });

    const el = render();
    expect(el.textContent).toContain('Vinicius');
    expect(el.textContent).not.toContain('Minha conta');
  });

  it('shows a distinct "Servidor indisponível" indicator, not the "Entrar" button', () => {
    auth.set({ status: 'unavailable' });

    const el = render();
    expect(el.textContent).toContain('Servidor indisponível');
    expect(el.textContent).not.toContain('Entrar');
    expect(el.querySelector('button')).toBeNull();
  });
});
