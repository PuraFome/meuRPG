import { Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/** "Névoa de guerra nos mapas novos" (RN-24): the app's switch, an ink track with a check when on, and its words beside it. */
@Component({
  selector: 'app-fog-switch',
  imports: [MatIconModule],
  templateUrl: './fog-switch.html',
  styleUrl: './fog-switch.scss',
})
export class FogSwitch {
  readonly checked = input.required<boolean>();
  readonly toggled = output<boolean>();
}
