import { Component, effect, inject, signal, untracked } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
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
 * "Meu perfil" (guarded by authGuard): sets the display name other members
 * of the caller's campaigns see (`IdentityService.UpdateProfile`). An empty
 * value clears it, per the proto's own doc comment.
 */
@Component({
  selector: 'app-profile',
  imports: [
    MatButtonModule,
    MatCardModule,
    MatFormFieldModule,
    MatInputModule,
    ReactiveFormsModule,
  ],
  templateUrl: './profile.html',
  styleUrl: './profile.scss',
})
export class Profile {
  private readonly auth = inject(AuthService);
  private readonly fb = inject(FormBuilder);

  protected readonly saveState = signal<SaveState>({ status: 'idle' });

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
