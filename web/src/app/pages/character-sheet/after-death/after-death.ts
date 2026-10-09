import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { RouterLink } from '@angular/router';

/**
 * "E agora?": what the owner of a dead character can do next. The campaign takes one living character per
 * player, and the death opens the way for a new one, which waits for the master's approval like any new sheet.
 * Shown only to the owner, and only while they have no living character of their own.
 */
@Component({
  selector: 'app-after-death',
  imports: [MatButtonModule, RouterLink],
  templateUrl: './after-death.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
  styleUrl: './after-death.scss',
})
export class AfterDeath {
  readonly characterName = input.required<string>();
  readonly campaignId = input.required<string>();
}
