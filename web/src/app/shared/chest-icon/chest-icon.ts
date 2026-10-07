import { ChangeDetectionStrategy, Component } from '@angular/core';

/**
 * The treasure chest of MAP-LANGUAGE.md ("Tesouro: o ícone do baú"): a rounded body with the lid line and the latch, drawn with
 * `currentColor` so it takes the ink of whatever holds it. 24 x 24, `aria-hidden`: the words around it carry the meaning.
 */
@Component({
  selector: 'app-chest-icon',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <svg viewBox="0 0 24 24" width="1em" height="1em" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <path d="M4 10a8 8 0 0 1 16 0v9H4z" />
      <path d="M4 12.5h16" />
      <rect x="10" y="11" width="4" height="4" rx="0.8" fill="currentColor" stroke="none" />
    </svg>
  `,
  styles:
    ':host { display: inline-flex; width: 1em; height: 1em; font-size: 24px; line-height: 1; } svg { display: block; }',
  host: { 'aria-hidden': 'true' },
})
export class ChestIcon {}
