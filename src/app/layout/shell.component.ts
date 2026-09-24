import { Component, inject, OnInit, OnDestroy, HostListener, computed } from '@angular/core';
import { RouterOutlet, RouterLink, RouterLinkActive, Router } from '@angular/router';
import { BreakpointObserver, Breakpoints } from '@angular/cdk/layout';
import { map } from 'rxjs/operators';
import { AsyncPipe } from '@angular/common';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatToolbarModule } from '@angular/material/toolbar';
import { MatListModule } from '@angular/material/list';
import { MatIconModule } from '@angular/material/icon';
import { MatButtonModule } from '@angular/material/button';
import { MatMenuModule } from '@angular/material/menu';
import { MatDialog, MatDialogRef } from '@angular/material/dialog';
import { SearchModalComponent } from '../shared/search-modal/search-modal.component';
import { AuthService } from '../core/auth/auth.service';

interface NavItem {
  path: string;
  label: string;
  icon: string;
}

@Component({
  selector: 'app-shell',
  standalone: true,
  imports: [
    RouterOutlet,
    RouterLink,
    RouterLinkActive,
    AsyncPipe,
    MatSidenavModule,
    MatToolbarModule,
    MatListModule,
    MatIconModule,
    MatButtonModule,
    MatMenuModule,
  ],
  template: `
    <mat-sidenav-container class="shell-container" [autosize]="true">
      <mat-sidenav
        #drawer
        class="app-sidenav"
        [mode]="(isHandset$ | async) ? 'over' : 'side'"
        [opened]="!((isHandset$ | async) ?? false)"
      >
        <mat-toolbar class="sidenav-header">
          <mat-icon>auto_stories</mat-icon>
          <span>MeuRPG</span>
        </mat-toolbar>

        <mat-nav-list>
          @for (item of navItems; track item.path) {
            <a
              mat-list-item
              [routerLink]="item.path"
              routerLinkActive="active-link"
              [routerLinkActiveOptions]="{ exact: item.path === '/' }"
              (click)="onNavClick(drawer)"
            >
              <mat-icon matListItemIcon>{{ item.icon }}</mat-icon>
              <span matListItemTitle>{{ item.label }}</span>
            </a>
          }
        </mat-nav-list>
      </mat-sidenav>

      <mat-sidenav-content>
        <mat-toolbar class="app-toolbar">
          <button
            mat-icon-button
            (click)="drawer.toggle()"
            aria-label="Alternar menu"
          >
            <mat-icon>menu</mat-icon>
          </button>

          <span class="toolbar-title">MeuRPG</span>

          <span class="spacer"></span>

          <button
            mat-icon-button
            (click)="openSearch()"
            aria-label="Pesquisar (Ctrl+K)"
          >
            <mat-icon>search</mat-icon>
          </button>

          @if (auth.user(); as user) {
            <button
              mat-icon-button
              class="user-menu-trigger"
              [matMenuTriggerFor]="userMenu"
              [attr.aria-label]="'Conta de ' + (user.name || user.email)"
            >
              <span class="user-avatar">{{ initials() }}</span>
            </button>
            <mat-menu #userMenu="matMenu">
              <div class="user-menu-header">
                <span class="user-menu-name">{{ user.name || 'Jogador' }}</span>
                <span class="user-menu-email">{{ user.email }}</span>
              </div>
              <button mat-menu-item routerLink="/perfil">
                <mat-icon>person</mat-icon>
                <span>Perfil</span>
              </button>
              <button mat-menu-item (click)="logout()">
                <mat-icon>logout</mat-icon>
                <span>Sair</span>
              </button>
            </mat-menu>
          } @else {
            <button mat-button routerLink="/login">
              <mat-icon>login</mat-icon>
              Entrar
            </button>
          }
        </mat-toolbar>

        <main class="content">
          <router-outlet />
        </main>
      </mat-sidenav-content>
    </mat-sidenav-container>
  `,
  styles: `
    :host {
      display: block;
      height: 100vh;
    }

    .shell-container {
      height: 100vh;
    }

    .app-sidenav {
      width: 240px;
      border-right: 1px solid rgba(255, 255, 255, 0.08);
    }

    .sidenav-header {
      display: flex;
      align-items: center;
      gap: 10px;
      font-size: 1.25rem;
      font-weight: 500;
      letter-spacing: 0.5px;
    }

    .app-toolbar {
      display: flex;
      align-items: center;
      gap: 8px;
      position: sticky;
      top: 0;
      z-index: 10;
    }

    .toolbar-title {
      font-size: 1.1rem;
      font-weight: 500;
    }

    .spacer {
      flex: 1 1 auto;
    }

    .user-avatar {
      width: 34px;
      height: 34px;
      border-radius: 50%;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      font-size: 0.8rem;
      font-weight: 600;
      color: #1a1a2e;
      background: linear-gradient(135deg, #e0e0e0 0%, #b388ff 100%);
    }

    .user-menu-header {
      display: flex;
      flex-direction: column;
      padding: 8px 16px 4px;
      min-width: 180px;
    }

    .user-menu-name {
      font-weight: 600;
    }

    .user-menu-email {
      font-size: 0.75rem;
      opacity: 0.6;
    }

    .content {
      padding: 24px;
      min-height: calc(100vh - 64px);
    }

    @media (max-width: 600px) {
      .content {
        padding: 16px;
      }
    }

    .active-link {
      background: rgba(255, 255, 255, 0.06);
    }

    .active-link .mat-icon {
      color: rgb(var(--mat-app-primary));
    }
  `,
})
export class ShellComponent implements OnInit, OnDestroy {
  private readonly dialog = inject(MatDialog);
  private readonly breakpointObserver = inject(BreakpointObserver);
  readonly auth = inject(AuthService);
  private readonly router = inject(Router);

  readonly initials = computed(() => {
    const name = this.auth.user()?.name?.trim();
    if (!name) {
      return '?';
    }
    return name
      .split(/\s+/)
      .slice(0, 2)
      .map((part) => part.charAt(0).toUpperCase())
      .join('');
  });

  private searchDialogRef: MatDialogRef<SearchModalComponent> | null = null;

  readonly isHandset$ = this.breakpointObserver
    .observe(Breakpoints.Handset)
    .pipe(map((result) => result.matches));

  readonly navItems: NavItem[] = [
    { path: '/', label: 'Início', icon: 'home' },
    { path: '/personagens', label: 'Personagens', icon: 'people' },
    { path: '/mapa', label: 'Mapa', icon: 'map' },
    { path: '/campanha', label: 'Campanha', icon: 'folder' },
    { path: '/galeria', label: 'Galeria', icon: 'collections_bookmark' },
    { path: '/regras', label: 'Regras', icon: 'menu_book' },
    { path: '/sessao', label: 'Sessão', icon: 'event' },
  ];

  private isHandsetValue = false;

  ngOnInit() {
    this.isHandset$.subscribe((matches) => {
      this.isHandsetValue = matches;
    });
  }

  ngOnDestroy() {
    // Cleanup handled by AsyncPipe
  }

  @HostListener('window:keydown', ['$event'])
  onKeydown(event: KeyboardEvent): void {
    if ((event.ctrlKey || event.metaKey) && event.key === 'k') {
      event.preventDefault();
      this.openSearch();
    }
  }

  openSearch(): void {
    if (this.searchDialogRef) {
      return;
    }
    this.searchDialogRef = this.dialog.open(SearchModalComponent, {
      width: '520px',
      maxWidth: '95vw',
      panelClass: 'search-dialog',
    });
    this.searchDialogRef.afterClosed().subscribe(() => {
      this.searchDialogRef = null;
    });
  }

  async logout(): Promise<void> {
    await this.auth.logout();
    await this.router.navigate(['/login']);
  }

  onNavClick(drawer: { close: () => void }): void {
    if (this.isHandsetValue) {
      drawer.close();
    }
  }
}
