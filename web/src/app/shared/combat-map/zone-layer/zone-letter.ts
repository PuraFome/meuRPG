import { ChangeDetectionStrategy, Component, input } from '@angular/core';

/**
 * A zone's letter in a white diamond, for the legend and the zones list: the same mark as the map's corner badge, so a letter
 * on the map finds its name beside it. It is a diamond, never a square: a square reads as an NPC token on the map.
 * The picture says "Zona A" to a screen reader.
 */
@Component({
  selector: 'app-zone-letter',
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `<span class="zlt" role="img" [attr.aria-label]="'Zona ' + letter()"><span class="zlt__l" aria-hidden="true">{{ letter() }}</span></span>`,
  styles: `
    :host {
      display: inline-flex;
      flex: none;
    }
    .zlt {
      position: relative;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 24px;
      height: 24px;
    }
    .zlt::before {
      content: '';
      position: absolute;
      inset: 4px;
      border: 1.5px solid var(--mr-map-token-ink);
      background: var(--mr-map-token-surface);
      transform: rotate(45deg);
    }
    .zlt__l {
      position: relative;
      color: var(--mr-map-token-ink);
      font-size: var(--mr-font-12);
      font-weight: 700;
      line-height: 1;
    }
  `,
})
export class ZoneLetter {
  readonly letter = input.required<string>();
}
