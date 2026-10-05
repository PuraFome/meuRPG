import { Component, inject, input, output, signal } from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import { openFamiliarEyes } from './familiar-eyes-sheet';

/**
 * "Ver pelos olhos" (MR-036, E9-04): the button of a familiar's row. It asks the
 * question (`app-familiar-eyes-sheet`) before it does anything, and tells the host
 * when the sight started (`started`); what changes on screen — the band, the map,
 * the blind line — comes from the stream. It knows nothing of where it sits, so the
 * creatures' list of the sheet (9.16) and the combat's order can both use it.
 *
 * It is offered for a familiar; the server decides whether it is near enough (30 m)
 * and says so when it is not: the browser never measures a distance.
 */
@Component({
  selector: 'app-familiar-eyes-button',
  host: { '[class.eyes--block]': 'block()' },
  imports: [MatButtonModule, MatIconModule],
  template: `
    <button mat-stroked-button type="button" class="eyes" [disabled]="disabled()" (click)="ask()">
      <mat-icon aria-hidden="true">visibility</mat-icon>Ver pelos olhos
    </button>
  `,
  styles: `
    :host {
      display: inline-block;
    }

    :host(.eyes--block) {
      display: block;
    }

    :host(.eyes--block) .eyes {
      width: 100%;
    }

    .eyes {
      min-height: 44px;
    }

    @media (max-width: 767.98px) {
      .eyes {
        min-height: 48px;
      }
    }
  `,
})
export class FamiliarEyesButton {
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);

  readonly campaignId = input.required<string>();
  readonly characterId = input.required<string>();
  readonly characterName = input.required<string>();
  /** The familiar's name ("Nanquim"). */
  readonly familiarName = input.required<string>();
  /** A combat is on: the question says the action is spent. */
  readonly inCombat = input(false);
  readonly disabled = input(false);
  /** As wide as its container (a card on a phone). */
  readonly block = input(false);

  /** The sight started. */
  readonly started = output<void>();
  protected readonly open = signal(false);

  protected ask(): void {
    if (this.open()) {
      return;
    }
    this.open.set(true);
    openFamiliarEyes(this.dialog, this.bottomSheet, {
      campaignId: this.campaignId(),
      characterId: this.characterId(),
      characterName: this.characterName(),
      familiarName: this.familiarName(),
      inCombat: this.inCombat(),
    }).subscribe((started) => {
      this.open.set(false);
      if (started) {
        this.started.emit();
      }
    });
  }
}
