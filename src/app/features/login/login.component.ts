import { Component, inject } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { AuthService } from '../../core/auth/auth.service';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [MatButtonModule, MatIconModule],
  template: `
    <div class="login-page">
      <div class="login-card">
        <div class="login-brand">
          <mat-icon class="login-brand-icon">auto_stories</mat-icon>
          <h1 class="login-title">MeuRPG</h1>
        </div>

        <h2 class="login-heading">Entrar no MeuRPG</h2>
        <p class="login-text">
          Use sua conta Google para acessar suas campanhas e personagens.
          Seus personagens ficam vinculados à sua conta.
        </p>

        <button
          mat-raised-button
          color="primary"
          class="login-cta"
          type="button"
          (click)="login()"
        >
          <mat-icon>login</mat-icon>
          Entrar com o Google
        </button>

        <p class="login-legal">
          Ao entrar, você concorda com o uso do seu nome, e-mail e foto de
          perfil do Google apenas para identificação na aplicação.
        </p>
      </div>
    </div>
  `,
  styles: `
    :host {
      --login-accent: #b388ff;
      --login-accent-soft: rgba(179, 136, 255, 0.16);
      --login-surface: rgba(255, 255, 255, 0.04);
      --login-border: rgba(255, 255, 255, 0.1);

      display: block;
      min-height: 100vh;
      background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
    }

    .login-page {
      min-height: 100vh;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 24px;
    }

    .login-card {
      width: 100%;
      max-width: 420px;
      padding: 40px 32px;
      text-align: center;
      border: 1px solid var(--login-border);
      border-radius: 16px;
      background:
        radial-gradient(120% 140% at 0% 0%, var(--login-accent-soft) 0%, transparent 60%),
        var(--login-surface);
      backdrop-filter: blur(8px);
      box-shadow: 0 20px 60px rgba(0, 0, 0, 0.45);
      color: #eceff4;
    }

    .login-brand {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 10px;
      margin-bottom: 24px;
    }

    .login-brand-icon {
      font-size: 2rem;
      width: 2rem;
      height: 2rem;
      color: var(--login-accent);
    }

    .login-title {
      margin: 0;
      font-size: 1.75rem;
      font-weight: 700;
      letter-spacing: 1px;
      background: linear-gradient(135deg, #e0e0e0 0%, var(--login-accent) 100%);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
      background-clip: text;
    }

    .login-heading {
      margin: 0 0 12px;
      font-size: 1.2rem;
      font-weight: 600;
    }

    .login-text {
      margin: 0 0 28px;
      font-size: 0.9rem;
      line-height: 1.6;
      opacity: 0.7;
    }

    .login-cta {
      width: 100%;
      height: 48px;
      font-size: 1rem;
    }

    .login-legal {
      margin: 24px 0 0;
      font-size: 0.72rem;
      line-height: 1.5;
      opacity: 0.45;
    }
  `,
})
export class LoginComponent {
  private readonly auth = inject(AuthService);

  login(): void {
    this.auth.login();
  }
}
