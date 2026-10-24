import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { RouterLink } from '@angular/router';

/**
 * The three panels that open a page of the table's rules: "Regras da mesa", "Conteúdo da mesa"
 * and "Magias". The master's "Regras" tab and the player's campaign page both draw them, each
 * with the words for the person who reads (the master edits, a player reads).
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-rule-links',
  imports: [MatButtonModule, RouterLink],
  templateUrl: './rule-links.html',
  styleUrl: './rule-links.scss',
})
export class RuleLinks {
  readonly campaignId = input.required<string>();
  readonly isMaster = input.required<boolean>();
}
