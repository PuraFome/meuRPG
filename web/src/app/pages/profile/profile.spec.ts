import { Injectable, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Code, ConnectError } from '@connectrpc/connect';

import { AuthState, AuthService } from '../../core/auth/auth.service';
import { Profile } from './profile';

@Injectable()
class FakeAuthService {
  private readonly stateSignal = signal<AuthState>({ status: 'unknown' });
  readonly state = this.stateSignal.asReadonly();
  readonly updateProfile = vi.fn().mockResolvedValue(undefined);

  set(state: AuthState): void {
    this.stateSignal.set(state);
  }
}

describe('Profile', () => {
  let auth: FakeAuthService;

  beforeEach(() => {
    TestBed.configureTestingModule({
      imports: [Profile],
      providers: [{ provide: AuthService, useClass: FakeAuthService }],
    });
    auth = TestBed.inject(AuthService) as unknown as FakeAuthService;
  });

  it('has exactly one h1, "Meu perfil"', () => {
    auth.set({
      status: 'signed-in',
      user: { id: 'u1', displayName: null },
      sessionExpiresAt: null,
    });
    const fixture = TestBed.createComponent(Profile);
    fixture.detectChanges();
    const headings = (fixture.nativeElement as HTMLElement).querySelectorAll('h1');
    expect(headings.length).toBe(1);
    expect(headings[0].textContent).toContain('Meu perfil');
  });

  it('seeds the form with the current display name', () => {
    auth.set({
      status: 'signed-in',
      user: { id: 'u1', displayName: 'Vinicius' },
      sessionExpiresAt: null,
    });
    const fixture = TestBed.createComponent(Profile);
    fixture.detectChanges();

    const input = (fixture.nativeElement as HTMLElement).querySelector('input') as HTMLInputElement;
    expect(input.value).toBe('Vinicius');
  });

  it('saves the new display name and shows a status message', async () => {
    auth.set({
      status: 'signed-in',
      user: { id: 'u1', displayName: null },
      sessionExpiresAt: null,
    });
    const fixture = TestBed.createComponent(Profile);
    fixture.detectChanges();

    await fixture.componentInstance['submit']();
    fixture.detectChanges();

    expect(auth.updateProfile).toHaveBeenCalledWith('');
    expect(
      (fixture.nativeElement as HTMLElement).querySelector('[role="status"]')?.textContent,
    ).toContain('atualizado');
  });

  it('trims the name before saving, so an empty (all-spaces) value clears it', async () => {
    auth.set({
      status: 'signed-in',
      user: { id: 'u1', displayName: null },
      sessionExpiresAt: null,
    });
    const fixture = TestBed.createComponent(Profile);
    fixture.detectChanges();

    fixture.componentInstance['form'].setValue({ displayName: '   ' });
    await fixture.componentInstance['submit']();

    expect(auth.updateProfile).toHaveBeenCalledWith('');
  });

  it('shows a clear message when saving fails with invalid_argument', async () => {
    auth.updateProfile.mockRejectedValue(new ConnectError('bad name', Code.InvalidArgument));
    auth.set({
      status: 'signed-in',
      user: { id: 'u1', displayName: null },
      sessionExpiresAt: null,
    });
    const fixture = TestBed.createComponent(Profile);
    fixture.detectChanges();

    await fixture.componentInstance['submit']();
    fixture.detectChanges();

    expect((fixture.nativeElement as HTMLElement).textContent).toContain('até 40 caracteres');
  });
});
