import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';

/**
 * "Davi já tem outro personagem vivo": the master's card when a revival is refused because the player made another
 * character after the death. It says the way out (the master files the living one or marks it dead first) and links
 * to that character; the app never does it by itself. A `role="alert"`, in the place of the question.
 */
@Component({
  selector: 'app-revive-blocked',
  imports: [MatButtonModule, MatIconModule, RouterLink],
  templateUrl: './revive-blocked.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
  styleUrl: './revive-blocked.scss',
})
export class ReviveBlocked {
  readonly deadName = input.required<string>();
  /** The player's name; "o jogador" when the page does not know it. */
  readonly playerName = input('o jogador');
  readonly livingName = input.required<string>();
  /** The router link of the living character's page. */
  readonly livingLink = input.required<readonly string[]>();
  readonly cancelled = output<void>();
}
