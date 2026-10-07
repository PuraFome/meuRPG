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
  readonly countOtherSessions = vi.fn().mockResolvedValue(0);
  readonly signOutOtherSessions = vi.fn().mockResolvedValue(0);

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

  describe('Sessões', () => {
    const signedIn: AuthState = {
      status: 'signed-in',
      user: { id: 'u1', displayName: null },
      sessionExpiresAt: null,
    };

    async function open(others: number) {
      auth.countOtherSessions.mockResolvedValue(others);
      auth.set(signedIn);
      const fixture = TestBed.createComponent(Profile);
      fixture.detectChanges();
      await fixture.whenStable();
      fixture.detectChanges();
      return fixture;
    }
    const text = (f: { nativeElement: unknown }) => (f.nativeElement as HTMLElement).textContent ?? '';
    const button = (f: { nativeElement: unknown }, label: string) =>
      Array.from((f.nativeElement as HTMLElement).querySelectorAll('button')).find((b) =>
        b.textContent?.includes(label),
      );

    it('says there is no other device, with no button', async () => {
      const fixture = await open(0);
      expect(text(fixture)).toContain('não está conectado em outros dispositivos');
      expect(button(fixture, 'Sair dos outros dispositivos')).toBeUndefined();
    });

    it('counts the other devices, singular and plural', async () => {
      expect(text(await open(1))).toContain('conectado em 1 outro dispositivo.');
      expect(text(await open(3))).toContain('conectado em 3 outros dispositivos.');
    });

    it('confirms in place, then signs out and shows the result', async () => {
      const fixture = await open(2);
      button(fixture, 'Sair dos outros dispositivos')!.click();
      fixture.detectChanges();
      // Nothing is sent before the confirmation.
      expect(auth.signOutOtherSessions).not.toHaveBeenCalled();
      expect(text(fixture)).toContain('Não dá para desfazer');

      auth.signOutOtherSessions.mockResolvedValue(2);
      button(fixture, 'Confirmar saída')!.click();
      await fixture.whenStable();
      fixture.detectChanges();

      expect(auth.signOutOtherSessions).toHaveBeenCalledTimes(1);
      expect(
        (fixture.nativeElement as HTMLElement).querySelector('[role="status"]')?.textContent,
      ).toContain('2 dispositivos foram desconectados');
    });

    it('cancelling sends nothing and goes back to the button', async () => {
      const fixture = await open(2);
      button(fixture, 'Sair dos outros dispositivos')!.click();
      fixture.detectChanges();
      button(fixture, 'Cancelar')!.click();
      fixture.detectChanges();

      expect(auth.signOutOtherSessions).not.toHaveBeenCalled();
      expect(button(fixture, 'Sair dos outros dispositivos')).toBeDefined();
    });

    it('shows an alert when the count cannot be read', async () => {
      auth.countOtherSessions.mockRejectedValue(new ConnectError('down', Code.Unavailable));
      auth.set(signedIn);
      const fixture = TestBed.createComponent(Profile);
      fixture.detectChanges();
      await fixture.whenStable();
      fixture.detectChanges();

      expect(
        (fixture.nativeElement as HTMLElement).querySelector('[role="alert"]')?.textContent,
      ).toContain('Não foi possível');
    });
  });
});
