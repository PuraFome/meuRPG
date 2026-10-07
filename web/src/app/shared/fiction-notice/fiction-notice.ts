import { Component } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

/**
 * The fixed notice PRIV-21 requires next to every free-text group where a
 * player or master could type something about a real person — personality,
 * appearance, backstory, allies, master notes, and any other free-text field
 * on a character. Always the same wording, so it lives in one component
 * instead of a string copied into every form section (`docs/privacy.md`).
 */
@Component({
  selector: 'app-fiction-notice',
  imports: [MatIconModule],
  templateUrl: './fiction-notice.html',
  styleUrl: './fiction-notice.scss',
})
export class FictionNotice {}
