import { Component, inject, OnInit, OnDestroy, HostListener } from '@angular/core';
import { RouterOutlet, RouterLink, RouterLinkActive } from '@angular/router';
import { BreakpointObserver, Breakpoints } from '@angular/cdk/layout';
import { map } from 'rxjs/operators';
import { AsyncPipe } from '@angular/common';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatToolbarModule } from '@angular/material/toolbar';
import { MatListModule } from '@angular/material/list';
import { MatIconModule } from '@angular/material/icon';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog, MatDialogRef } from '@angular/material/dialog';
import { SearchModalComponent } from '../shared/search-modal/search-modal.component';

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

    .content {
      padding: 24px;
      min-height: calc(100vh - 64px);
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

  onNavClick(drawer: { close: () => void }): void {
    if (this.isHandsetValue) {
      drawer.close();
    }
  }
}
