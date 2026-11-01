import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * The warning of an area spell step ("Toren está na área." or "Ninguém que você vê está na área."). It has no live role: the
 * screen draws it inside a `role="status"` region that is already in the page, because the step brings the text in later.
 */
@Component({
  selector: 'app-area-notice',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  template: `
    <div class="mr-notice mr-notice--warning">
      <mat-icon aria-hidden="true">warning</mat-icon>
      <p><strong>{{ notice().strong }}</strong> {{ notice().rest }}</p>
    </div>
  `,
  styles: `
    :host {
      display: block;
    }
  `,
})
export class AreaNotice {
  readonly notice = input.required<{ readonly strong: string; readonly rest: string }>();
}
