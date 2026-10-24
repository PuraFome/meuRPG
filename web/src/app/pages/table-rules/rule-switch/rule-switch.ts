import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * A yes/no table rule (RN-24): the app's switch, an ink track with a check when on, and its words beside it. `key` makes
 * the ids the switch is named and described by, so two switches on one page never share one.
 */
@Component({
  selector: 'app-rule-switch',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatIconModule],
  templateUrl: './rule-switch.html',
  styleUrl: './rule-switch.scss',
})
export class RuleSwitch {
  readonly key = input.required<string>();
  readonly label = input.required<string>();
  readonly hint = input.required<string>();
  readonly checked = input.required<boolean>();
  readonly toggled = output<boolean>();
}
