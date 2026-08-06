import { Component, inject, HostListener } from '@angular/core';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { SessionQuickSearchComponent } from './session-quick-search.component';

@Component({
  selector: 'app-session',
  standalone: true,
  imports: [MatIconModule],
  template: `
    <div class="session-shell">
      <div class="session-topbar">
        <div class="session-title-row">
          <mat-icon class="session-title-icon">event</mat-icon>
          <span class="session-title">Sessão</span>
        </div>
        <button
          mat-icon-button
          class="session-search-btn"
          (click)="openQuickSearch()"
          aria-label="Pesquisa rápida"
        >
          <mat-icon>search</mat-icon>
        </button>
      </div>
      <div class="session-body">
        <p class="session-placeholder">Sessão — em construção</p>
        <p class="session-hint">
          Pressione <kbd>Ctrl+F</kbd> para pesquisa rápida
        </p>
      </div>
    </div>
  `,
  styles: [
    `
      .session-shell {
        display: flex;
        flex-direction: column;
        height: 100%;
      }

      .session-topbar {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 8px 16px;
        border-bottom: 1px solid rgba(255, 255, 255, 0.08);
      }

      .session-title-row {
        display: flex;
        align-items: center;
        gap: 8px;
      }

      .session-title-icon {
        font-size: 1.2rem;
        width: 1.2rem;
        height: 1.2rem;
        opacity: 0.8;
        color: rgb(var(--mat-app-primary, 63, 81, 181));
      }

      .session-title {
        font-size: 1.1rem;
        font-weight: 600;
      }

      .session-search-btn {
        width: 36px;
        height: 36px;
        line-height: 36px;
      }

      .session-body {
        flex: 1;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        gap: 8px;
      }

      .session-placeholder {
        opacity: 0.5;
        font-size: 1rem;
      }

      .session-hint {
        font-size: 0.78rem;
        opacity: 0.35;
      }
      .session-hint kbd {
        display: inline-block;
        padding: 1px 5px;
        font-size: 0.72rem;
        font-family: inherit;
        background: rgba(255, 255, 255, 0.08);
        border: 1px solid rgba(255, 255, 255, 0.12);
        border-radius: 4px;
        color: rgba(255, 255, 255, 0.5);
      }
    `,
  ],
})
export class SessionComponent {
  private readonly dialog = inject(MatDialog);

  @HostListener('window:keydown', ['$event'])
  handleKeyboardEvent(event: KeyboardEvent): void {
    if ((event.ctrlKey || event.metaKey) && event.key === 'f') {
      event.preventDefault();
      this.openQuickSearch();
    }
  }

  openQuickSearch(): void {
    const dialogRef = this.dialog.open(SessionQuickSearchComponent, {
      panelClass: 'session-quick-search-panel',
      autoFocus: false,
      backdropClass: 'session-quick-search-backdrop',
    });
    dialogRef.afterClosed().subscribe();
  }
}
