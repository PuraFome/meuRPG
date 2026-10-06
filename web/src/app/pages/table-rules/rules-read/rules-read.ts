import { Component, input } from '@angular/core';

/**
 * "Regras da mesa" as a player reads it (MR-025, RN-24): the style and each rule in words, and the table's reminders,
 * with no controls. Only the master changes them; the lines come from the page, which has the words of every rule.
 */
@Component({
  selector: 'app-rules-read',
  templateUrl: './rules-read.html',
  styleUrl: './rules-read.scss',
})
export class RulesRead {
  readonly style = input.required<string>();
  readonly rows = input.required<readonly { readonly label: string; readonly value: string }[]>();
  readonly reminders = input.required<readonly string[]>();
}
