import { Component, Input } from '@angular/core';
import { RouterLink } from '@angular/router';
import { MatToolbarModule } from '@angular/material/toolbar';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

export interface BreadcrumbItem {
  label: string;
  route?: string;
}

@Component({
  selector: 'app-page-header',
  standalone: true,
  imports: [
    RouterLink,
    MatToolbarModule,
    MatButtonModule,
    MatIconModule,
  ],
  template: `
    <mat-toolbar class="page-header">
      <div class="header-left">
        @if (breadcrumbs.length) {
          <nav class="breadcrumbs">
            @for (crumb of breadcrumbs; track $index) {
              @if (crumb.route) {
                <a
                  class="breadcrumb-link"
                  [routerLink]="crumb.route"
                >
                  {{ crumb.label }}
                </a>
                <mat-icon class="breadcrumb-sep"
                  >chevron_right</mat-icon
                >
              } @else {
                <span class="breadcrumb-current">{{
                  crumb.label
                }}</span>
              }
            }
          </nav>
        }
        <div class="header-title-row">
          @if (icon) {
            <mat-icon class="header-icon" aria-hidden="true">{{ icon }}</mat-icon>
          }
          <span class="header-title">{{ title }}</span>
        </div>
      </div>
      <div class="header-actions">
        <ng-content select="[actions]" />
      </div>
    </mat-toolbar>
  `,
  styles: [
    `
      .page-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        background: transparent;
        padding: 0 4px;
        height: auto;
        min-height: 56px;
      }
      .header-left {
        display: flex;
        flex-direction: column;
        gap: 2px;
      }
      .breadcrumbs {
        display: flex;
        align-items: center;
        gap: 2px;
        font-size: 0.8rem;
        opacity: 0.65;
      }
      .breadcrumb-link {
        color: inherit;
        text-decoration: none;
        opacity: 0.7;
        transition: opacity 0.15s;
      }
      .breadcrumb-link:hover {
        opacity: 1;
        text-decoration: underline;
      }
      .breadcrumb-current {
        opacity: 0.5;
      }
      .breadcrumb-sep {
        font-size: 1rem;
        width: 1rem;
        height: 1rem;
        opacity: 0.4;
      }
      .header-title-row {
        display: flex;
        align-items: center;
        gap: 8px;
      }
      .header-icon {
        font-size: 1.4rem;
        width: 1.4rem;
        height: 1.4rem;
        opacity: 0.85;
        color: rgb(var(--mat-app-primary, 63, 81, 181));
      }
      .header-title {
        font-size: 1.25rem;
        font-weight: 500;
        letter-spacing: 0.25px;
      }
      .header-actions {
        display: flex;
        align-items: center;
        gap: 4px;
      }
    `,
  ],
})
export class PageHeaderComponent {
  @Input() title = '';
  @Input() icon = '';
  @Input() breadcrumbs: BreadcrumbItem[] = [];
}
