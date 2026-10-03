import { Component } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * "Pode subir de nível" (RN-12): an arrow and the words, on the success
 * surface, never the colour alone. The same tag on the campaign's group list,
 * the "Experiência" rows and the character sheet.
 */
@Component({
  selector: 'app-level-up-tag',
  imports: [MatIconModule],
  template: `<mat-icon aria-hidden="true">arrow_upward</mat-icon>Pode subir de nível`,
  styles: `
    :host {
      display: inline-flex;
      flex-shrink: 0;
      align-items: center;
      gap: 6px;
      box-sizing: border-box;
      height: 28px;
      padding: 0 10px;
      border-radius: var(--mr-radius-pill);
      background: var(--mr-success-surface);
      color: var(--mr-success-ink);
      font-size: 14px;
      font-weight: 700;
      line-height: 1;
      white-space: nowrap;
    }

    .mat-icon {
      width: 14px;
      height: 14px;
      font-size: 14px;
    }
  `,
})
export class LevelUpTag {}
