import { ChangeDetectionStrategy, Component, effect } from '@angular/core';

import type { TokenMover } from '../../../core/maps/token-mover';
import { SheetFrame } from '../../../shared/sheet/sheet-frame/sheet-frame';
import { injectSheet } from '../../../shared/sheet/sheet-host';
import { TokenMovePanel } from '../session-tokens/token-move-panel';

export interface PlayerMoveSheetData {
  readonly mover: TokenMover;
  readonly name: string;
  /** The master's switch, as the map says it now: when it goes off the sheet closes without moving (C9-03). */
  readonly canMove: () => boolean;
}

/**
 * "Mover Brisa" on a phone (C9-02, C9-03): the same panel as the master's in a sheet that covers the screen, with its filled button.
 * "Cancelar" and the close button put it away; the master taking the switch back closes it too, and the plan is lost.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-player-move-sheet',
  imports: [SheetFrame, TokenMovePanel],
  template: `
    <app-sheet-frame
      [title]="'Mover ' + data.name"
      titleId="move-title"
      [phone]="sheet.inSheet"
      [handle]="false"
      (closed)="close()"
    >
      <app-token-move-panel [mover]="data.mover" [sheet]="true" (closed)="close()" />
    </app-sheet-frame>
  `,
})
export class PlayerMoveSheet {
  protected readonly sheet = injectSheet<PlayerMoveSheetData, undefined>();
  protected readonly data = this.sheet.data;

  constructor() {
    effect(() => {
      if (!this.data.canMove()) {
        this.close();
      }
    });
  }

  protected close(): void {
    this.sheet.close(undefined);
  }
}
