import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';

import { RuleSwitch } from '../rule-switch/rule-switch';

/** "Névoa de guerra nos mapas novos" (RN-24): the rule switch with the fog's words. */
@Component({
  selector: 'app-fog-switch',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [RuleSwitch],
  templateUrl: './fog-switch.html',
})
export class FogSwitch {
  readonly checked = input.required<boolean>();
  readonly toggled = output<boolean>();
}
