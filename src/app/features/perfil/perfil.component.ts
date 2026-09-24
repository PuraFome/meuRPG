import { Component, computed, inject, signal } from '@angular/core';
import { Router } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { AuthService } from '../../core/auth/auth.service';
import {
  ConfirmDialogComponent,
  type ConfirmDialogData,
} from '../../shared/components/confirm-dialog.component';

@Component({
  selector: 'app-perfil',
  standalone: true,
  imports: [MatButtonModule, MatCardModule, MatIconModule],
  template: `
    <div class="perfil-page">
      <h1 class="perfil-title">Meu perfil</h1>

      @if (user(); as u) {
        <mat-card appearance="outlined" class="perfil-card">
          <mat-card-content class="perfil-content">
            <div class="perfil-avatar" aria-hidden="true">{{ initials() }}</div>
            <div class="perfil-info">
              <p class="perfil-name">{{ u.name || 'Jogador' }}</p>
              <p class="perfil-email">
                <mat-icon class="perfil-icon">mail</mat-icon>
                {{ u.email || 'e-mail não informado' }}
              </p>
            </div>
          </mat-card-content>
        </mat-card>

        <div class="perfil-actions">
          <button mat-stroked-button type="button" (click)="logout()">
            <mat-icon>logout</mat-icon>
            Sair
          </button>
          <button
            mat-stroked-button
            color="warn"
            type="button"
            [disabled]="deleting()"
            (click)="confirmDelete()"
          >
            <mat-icon>delete_forever</mat-icon>
            Excluir conta
          </button>
        </div>

        @if (error()) {
          <p class="perfil-error" role="alert">{{ error() }}</p>
        }
      } @else {
        <p class="perfil-empty">Você não está autenticado.</p>
      }
    </div>
  `,
  styles: `
    :host {
      --perfil-accent: #b388ff;
      --perfil-surface: rgba(255, 255, 255, 0.04);
      --perfil-border: rgba(255, 255, 255, 0.1);
      --perfil-danger: #f87171;

      display: block;
      max-width: 640px;
      margin: 0 auto;
    }

    .perfil-title {
      margin: 0 0 20px;
      font-size: 1.5rem;
      font-weight: 600;
    }

    .perfil-card {
      border: 1px solid var(--perfil-border);
      border-radius: 12px;
      background: var(--perfil-surface);
      margin-bottom: 20px;
    }

    .perfil-content {
      display: flex;
      align-items: center;
      gap: 20px;
      padding: 20px;
    }

    .perfil-avatar {
      width: 64px;
      height: 64px;
      flex-shrink: 0;
      display: flex;
      align-items: center;
      justify-content: center;
      border-radius: 50%;
      font-size: 1.5rem;
      font-weight: 600;
      color: #1a1a2e;
      background: linear-gradient(135deg, #e0e0e0 0%, var(--perfil-accent) 100%);
    }

    .perfil-name {
      margin: 0 0 6px;
      font-size: 1.15rem;
      font-weight: 600;
    }

    .perfil-email {
      display: flex;
      align-items: center;
      gap: 6px;
      margin: 0;
      font-size: 0.875rem;
      opacity: 0.65;
    }

    .perfil-icon {
      font-size: 1rem;
      width: 1rem;
      height: 1rem;
    }

    .perfil-actions {
      display: flex;
      gap: 12px;
      flex-wrap: wrap;
    }

    .perfil-error {
      margin: 16px 0 0;
      font-size: 0.875rem;
      color: var(--perfil-danger);
    }

    .perfil-empty {
      opacity: 0.6;
    }
  `,
})
export class PerfilComponent {
  private readonly auth = inject(AuthService);
  private readonly router = inject(Router);
  private readonly dialog = inject(MatDialog);

  readonly user = this.auth.user;
  readonly deleting = signal(false);
  readonly error = signal<string | null>(null);

  readonly initials = computed(() => {
    const name = this.user()?.name?.trim();
    if (!name) {
      return '?';
    }
    return name
      .split(/\s+/)
      .slice(0, 2)
      .map((part) => part.charAt(0).toUpperCase())
      .join('');
  });

  async logout(): Promise<void> {
    await this.auth.logout();
    await this.router.navigate(['/login']);
  }

  async confirmDelete(): Promise<void> {
    const data: ConfirmDialogData = {
      title: 'Excluir conta',
      message:
        'Todos os seus personagens e dados serão removidos permanentemente. Esta ação não pode ser desfeita.',
      confirmText: 'Excluir',
      cancelText: 'Cancelar',
    };
    const confirmed = await this.dialog
      .open(ConfirmDialogComponent, { data, width: '360px' })
      .afterClosed()
      .toPromise();
    if (confirmed) {
      await this.deleteAccount();
    }
  }

  private async deleteAccount(): Promise<void> {
    this.deleting.set(true);
    this.error.set(null);
    try {
      await this.auth.deleteAccount();
      await this.router.navigate(['/login']);
    } catch {
      this.error.set('Não foi possível excluir a conta. Tente novamente.');
      this.deleting.set(false);
    }
  }
}
