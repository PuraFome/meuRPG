import { ChangeDetectorRef, Component, inject, type OnDestroy } from '@angular/core';
import { DatePipe } from '@angular/common';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import {
  PageHeaderComponent,
  type BreadcrumbItem,
} from '../../shared/components/page-header.component';
import { CharactersService } from '../../core/services/characters.service';
import { environment } from '../../../environments/environment';

/** An invite link produced during the current session. */
export interface GeneratedInvite {
  token: string;
  link: string;
  expiresAt: string;
}

const GENERATE_ERROR = 'Não foi possível gerar o link. Tente novamente.';
const COPY_ERROR = 'Não foi possível copiar. Selecione o link e copie manualmente.';
const COPY_FEEDBACK_MS = 2000;

@Component({
  selector: 'app-character-invites',
  standalone: true,
  imports: [
    DatePipe,
    MatButtonModule,
    MatCardModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatProgressSpinnerModule,
    PageHeaderComponent,
  ],
  template: `
    <div class="invites-page">
      <app-page-header
        title="Convidar jogador"
        icon="person_add"
        [breadcrumbs]="breadcrumbs"
      />

      <section class="invite-hero">
        <div class="invite-hero-copy">
          <h2 class="invite-hero-title">Gere um link de convite</h2>
          <p class="invite-hero-text">
            Envie o link para o jogador. Ele entra com a conta Google, preenche a
            ficha e o personagem fica salvo na conta dele.
          </p>
        </div>

        <button
          mat-raised-button
          color="primary"
          type="button"
          class="invite-cta"
          [disabled]="generating"
          (click)="generate()"
        >
          @if (generating) {
            <mat-spinner class="invite-cta-spinner" [diameter]="18" />
          } @else {
            <mat-icon>send</mat-icon>
          }
          Enviar para jogador
        </button>
      </section>

      @if (error) {
        <div class="invite-error" role="alert">
          <mat-icon class="invite-error-icon" aria-hidden="true">error_outline</mat-icon>
          <span>{{ error }}</span>
        </div>
      }

      @if (links.length) {
        <div class="invite-list">
          @for (invite of links; track invite.token) {
            <mat-card appearance="outlined" class="invite-card">
              <mat-card-content class="invite-card-content">
                <div class="invite-card-meta">
                  <mat-icon class="invite-card-icon" aria-hidden="true">link</mat-icon>
                  <span class="invite-card-label">Link de convite</span>
                  <span class="invite-card-expiry">
                    expira em {{ invite.expiresAt | date: 'dd/MM/yyyy HH:mm' }}
                  </span>
                </div>

                <div class="invite-card-row">
                  <mat-form-field
                    appearance="outline"
                    class="invite-link-field"
                    subscriptSizing="dynamic"
                  >
                    <input
                      matInput
                      readonly
                      aria-label="Link de convite"
                      [value]="invite.link"
                    />
                  </mat-form-field>

                  <button
                    mat-stroked-button
                    type="button"
                    class="invite-copy"
                    (click)="copy(invite)"
                  >
                    <mat-icon>{{
                      copiedToken === invite.token ? 'check' : 'content_copy'
                    }}</mat-icon>
                    Copiar
                  </button>
                </div>

                @if (copied && copiedToken === invite.token) {
                  <p class="invite-copied" role="status">Link copiado!</p>
                }
              </mat-card-content>
            </mat-card>
          }
        </div>
      } @else if (!generating) {
        <p class="invite-hint">Nenhum link gerado nesta sessão ainda.</p>
      }
    </div>
  `,
  styles: `
    /* Tokens extracted from the app's M3 dark / violet-palette system. */
    :host {
      --inv-accent: #b388ff;
      --inv-accent-soft: rgba(179, 136, 255, 0.16);
      --inv-accent-faint: rgba(179, 136, 255, 0.08);
      --inv-surface: rgba(255, 255, 255, 0.04);
      --inv-border: rgba(255, 255, 255, 0.06);
      --inv-border-strong: rgba(255, 255, 255, 0.14);
      --inv-danger: #f87171;
      --inv-danger-surface: rgba(220, 38, 38, 0.12);
      --inv-radius-card: 12px;
      --inv-radius-inner: 10px;
      --inv-shadow-raised: 0 8px 24px rgba(0, 0, 0, 0.35);
      --inv-space-xs: 4px;
      --inv-space-sm: 8px;
      --inv-space-md: 12px;
      --inv-space-lg: 16px;
      --inv-space-xl: 24px;
      --inv-motion: 0.2s ease;

      display: block;
      max-width: 800px;
      margin: 0 auto;
    }

    .invites-page {
      padding: var(--inv-space-sm) 0;
    }

    /* ── Hero / primary action ─────────────────── */

    .invite-hero {
      position: relative;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: var(--inv-space-xl);
      flex-wrap: wrap;
      margin: var(--inv-space-lg) 0 var(--inv-space-xl);
      padding: var(--inv-space-xl);
      border: 1px solid var(--inv-border-strong);
      border-radius: var(--inv-radius-card);
      background:
        radial-gradient(
          120% 140% at 0% 0%,
          var(--inv-accent-soft) 0%,
          transparent 60%
        ),
        var(--inv-surface);
      overflow: hidden;
    }

    .invite-hero-copy {
      flex: 1;
      min-width: 240px;
    }

    .invite-hero-title {
      margin: 0 0 var(--inv-space-sm);
      font-size: 1.25rem;
      font-weight: 600;
      letter-spacing: 0.25px;
      background: linear-gradient(135deg, #e0e0e0 0%, var(--inv-accent) 100%);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
      background-clip: text;
    }

    .invite-hero-text {
      margin: 0;
      font-size: 0.875rem;
      line-height: 1.5;
      opacity: 0.6;
      max-width: 44ch;
    }

    .invite-cta {
      flex-shrink: 0;
      transition: transform var(--inv-motion), box-shadow var(--inv-motion);
    }

    .invite-cta:not(:disabled):hover {
      transform: translateY(-2px);
      box-shadow: var(--inv-shadow-raised);
    }

    .invite-cta-spinner {
      margin-right: var(--inv-space-sm);
    }

    /* ── Error ─────────────────────────────────── */

    .invite-error {
      display: flex;
      align-items: center;
      gap: var(--inv-space-sm);
      margin-bottom: var(--inv-space-lg);
      padding: var(--inv-space-md) var(--inv-space-lg);
      border: 1px solid var(--inv-danger);
      border-radius: var(--inv-radius-inner);
      background: var(--inv-danger-surface);
      color: var(--inv-danger);
      font-size: 0.875rem;
    }

    .invite-error-icon {
      font-size: 1.25rem;
      width: 1.25rem;
      height: 1.25rem;
      flex-shrink: 0;
    }

    /* ── Generated links ───────────────────────── */

    .invite-list {
      display: flex;
      flex-direction: column;
      gap: var(--inv-space-lg);
    }

    .invite-card {
      border: 1px solid var(--inv-border);
      border-radius: var(--inv-radius-card);
      transition: border-color var(--inv-motion), box-shadow var(--inv-motion);
      animation: invite-in var(--inv-motion) both;
    }

    .invite-card:hover {
      border-color: var(--inv-border-strong);
      box-shadow: var(--inv-shadow-raised);
    }

    .invite-card-content {
      padding: var(--inv-space-lg);
    }

    .invite-card-meta {
      display: flex;
      align-items: center;
      gap: var(--inv-space-sm);
      margin-bottom: var(--inv-space-md);
      flex-wrap: wrap;
    }

    .invite-card-icon {
      font-size: 1.125rem;
      width: 1.125rem;
      height: 1.125rem;
      color: var(--inv-accent);
    }

    .invite-card-label {
      font-size: 0.75rem;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      padding: 2px 10px;
      border-radius: var(--inv-space-md);
      background: var(--inv-accent-faint);
      color: var(--inv-accent);
    }

    .invite-card-expiry {
      font-size: 0.75rem;
      opacity: 0.5;
    }

    .invite-card-row {
      display: flex;
      align-items: center;
      gap: var(--inv-space-md);
      flex-wrap: wrap;
    }

    .invite-link-field {
      flex: 1;
      min-width: 220px;
    }

    .invite-copy {
      flex-shrink: 0;
    }

    .invite-copied {
      margin: var(--inv-space-md) 0 0;
      font-size: 0.75rem;
      font-weight: 600;
      letter-spacing: 0.5px;
      color: var(--inv-accent);
      animation: invite-in var(--inv-motion) both;
    }

    /* ── Empty hint ────────────────────────────── */

    .invite-hint {
      margin: 0;
      font-size: 0.875rem;
      opacity: 0.45;
      text-align: center;
      padding: var(--inv-space-xl) 0;
    }

    @keyframes invite-in {
      from {
        opacity: 0;
        transform: translateY(var(--inv-space-sm));
      }
      to {
        opacity: 1;
        transform: translateY(0);
      }
    }

    @media (prefers-reduced-motion: reduce) {
      .invite-card,
      .invite-copied {
        animation: none;
      }

      .invite-cta,
      .invite-card {
        transition: none;
      }

      .invite-cta:not(:disabled):hover {
        transform: none;
      }
    }
  `,
})
export class CharacterInvitesComponent implements OnDestroy {
  private readonly characters = inject(CharactersService);
  // App is zoneless: async subscribe callbacks must trigger CD explicitly.
  private readonly cdr = inject(ChangeDetectorRef);

  readonly breadcrumbs: BreadcrumbItem[] = [
    { label: 'Personagens', route: '/personagens' },
    { label: 'Convidar jogador' },
  ];

  /** True while a join token request is in flight. */
  generating = false;

  /** User-facing failure message, or null when there is none. */
  error: string | null = null;

  /** Transient "copied" feedback flag. */
  copied = false;

  /** Token of the link that was just copied, for per-row feedback. */
  copiedToken: string | null = null;

  /** Invite links generated during this session (newest first). */
  links: GeneratedInvite[] = [];

  private copyTimer: ReturnType<typeof setTimeout> | null = null;

  /** Request a join token and turn it into a shareable hash URL. */
  generate(): void {
    this.generating = true;
    this.error = null;

    this.characters.createJoinToken().subscribe({
      next: (info) => {
        const link = environment.inviteBaseUrl + '#/personagens/convidar/' + info.token;
        this.links.unshift({
          token: info.token,
          link,
          expiresAt: info.expiresAt,
        });
        this.generating = false;
        this.cdr.detectChanges();
      },
      error: () => {
        this.error = GENERATE_ERROR;
        this.generating = false;
        this.cdr.detectChanges();
      },
    });
  }

  /** Copy an invite link to the clipboard, with transient confirmation. */
  copy(invite: GeneratedInvite): void {
    const clipboard: Clipboard | undefined = navigator.clipboard;

    if (!clipboard || typeof clipboard.writeText !== 'function') {
      this.error = COPY_ERROR;
      return;
    }

    clipboard.writeText(invite.link).then(
      () => this.markCopied(invite.token),
      () => {
        this.error = COPY_ERROR;
        this.cdr.detectChanges();
      },
    );
  }

  ngOnDestroy(): void {
    this.clearCopyTimer();
  }

  private markCopied(token: string): void {
    this.error = null;
    this.copied = true;
    this.copiedToken = token;
    this.clearCopyTimer();
    this.cdr.detectChanges();
    this.copyTimer = setTimeout(() => {
      this.copied = false;
      this.copiedToken = null;
      this.copyTimer = null;
      this.cdr.detectChanges();
    }, COPY_FEEDBACK_MS);
  }

  private clearCopyTimer(): void {
    if (this.copyTimer !== null) {
      clearTimeout(this.copyTimer);
      this.copyTimer = null;
    }
  }
}
