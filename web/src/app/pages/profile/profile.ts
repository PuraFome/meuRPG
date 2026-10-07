import {
  Component,
  ElementRef,
  Injector,
  afterNextRender,
  effect,
  inject,
  computed,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { Code } from '@connectrpc/connect';

import { AuthService } from '../../core/auth/auth.service';
import { describeConnectError } from '../../core/connect/connect-errors';

type SaveState =
  | { status: 'idle' }
  | { status: 'saving' }
  | { status: 'saved' }
  | { status: 'error'; message: string };

/**
 * The "Sessões" section: how many other devices are signed in, and the
 * in-place confirmation of "Sair dos outros dispositivos" (docs/design.md:
 * what can't be undone is confirmed on the screen itself).
 */
type SessionsState =
  | { status: 'loading' }
  | { status: 'ready'; others: number }
  | { status: 'confirm'; others: number }
  | { status: 'ending'; others: number }
  | { status: 'done'; ended: number }
  | { status: 'error'; message: string; others: number | null };

/**
 * "Meu perfil" (guarded by authGuard): sets the display name other members
 * of the caller's campaigns see (`IdentityService.UpdateProfile`). An empty
 * value clears it, per the proto's own doc comment. It also lists how many
 * other devices are signed in and can sign them out
 * (`IdentityService.SignOutOtherSessions`).
 */
@Component({
  selector: 'app-profile',
  imports: [MatButtonModule, MatFormFieldModule, MatIconModule, MatInputModule, ReactiveFormsModule],
  templateUrl: './profile.html',
  styleUrl: './profile.scss',
})
export class Profile {
  private readonly auth = inject(AuthService);
  private readonly fb = inject(FormBuilder);
  private readonly injector = inject(Injector);

  protected readonly saveState = signal<SaveState>({ status: 'idle' });

  protected readonly sessions = signal<SessionsState>({ status: 'loading' });

  // The template's @switch cannot narrow the union, so these read it.
  protected readonly sessionOthers = computed(() => {
    const s = this.sessions();
    return 'others' in s ? (s.others ?? 0) : 0;
  });
  protected readonly doneMessage = computed(() => {
    const s = this.sessions();
    if (s.status !== 'done' || s.ended === 0) {
      return 'Nenhum outro dispositivo estava conectado.';
    }
    return s.ended === 1
      ? 'Pronto: 1 dispositivo foi desconectado.'
      : `Pronto: ${s.ended} dispositivos foram desconectados.`;
  });
  protected readonly errorMessage = computed(() => {
    const s = this.sessions();
    return s.status === 'error' ? s.message : '';
  });

  private readonly confirmButton = viewChild('confirmButton', {
    read: ElementRef<HTMLButtonElement>,
  });
  private readonly askButton = viewChild('askButton', { read: ElementRef<HTMLButtonElement> });

  protected readonly form = this.fb.nonNullable.group({
    displayName: ['', Validators.maxLength(40)],
  });

  constructor() {
    // Seeds the form once the signed-in user (and their current display
    // name) is known; AuthService.state starts as 'unknown' and this page
    // is behind authGuard, so it always settles to 'signed-in' here.
    effect(() => {
      const state = this.auth.state();
      if (state.status === 'signed-in') {
        untracked(() => this.form.setValue({ displayName: state.user.displayName ?? '' }));
      }
    });
    void this.loadSessions();
  }

  /** Reads how many other sessions work; the "Sessões" section waits on it. */
  protected async loadSessions(): Promise<void> {
    try {
      this.sessions.set({ status: 'ready', others: await this.auth.countOtherSessions() });
    } catch (err) {
      this.sessions.set({
        status: 'error',
        message: describeConnectError(err, {}),
        others: null,
      });
    }
  }

  protected askSignOutOthers(others: number): void {
    this.sessions.set({ status: 'confirm', others });
    // The confirmation takes the focus, so a keyboard user lands on it.
    afterNextRender(() => this.confirmButton()?.nativeElement.focus(), {
      injector: this.injector,
    });
  }

  protected cancelSignOutOthers(others: number): void {
    this.sessions.set({ status: 'ready', others });
    afterNextRender(() => this.askButton()?.nativeElement.focus(), { injector: this.injector });
  }

  protected async signOutOthers(others: number): Promise<void> {
    this.sessions.set({ status: 'ending', others });
    try {
      this.sessions.set({ status: 'done', ended: await this.auth.signOutOtherSessions() });
    } catch (err) {
      this.sessions.set({ status: 'error', message: describeConnectError(err, {}), others });
    }
  }

  protected async submit(): Promise<void> {
    if (this.form.invalid) {
      this.form.markAllAsTouched();
      return;
    }
    this.saveState.set({ status: 'saving' });
    try {
      await this.auth.updateProfile(this.form.getRawValue().displayName.trim());
      this.saveState.set({ status: 'saved' });
    } catch (err) {
      this.saveState.set({
        status: 'error',
        message: describeConnectError(err, {
          [Code.InvalidArgument]: 'O nome deve ter até 40 caracteres, sem quebras de linha.',
        }),
      });
    }
  }
}
