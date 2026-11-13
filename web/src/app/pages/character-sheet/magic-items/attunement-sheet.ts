import { ChangeDetectionStrategy, Component, type Signal } from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';

import type { AttunementVm } from '../../../core/characters/attunement';
import { SheetFrame } from '../../../shared/sheet/sheet-frame/sheet-frame';
import { injectSheet, openSheet } from '../../../shared/sheet/sheet-host';
import { AttunementSection } from '../attunement-section/attunement-section';

/** What the sheet "Sintonização" needs: the signals follow the page, which reads the sheet again after each tap. */
export interface AttunementSheetData {
  readonly campaignId: string;
  readonly characterId: string;
  readonly characterName: string;
  readonly isMaster: boolean;
  readonly dead: boolean;
  readonly attunement: Signal<AttunementVm>;
  readonly revision: Signal<number>;
  readonly windowOpen: Signal<boolean>;
  /** An attunement was set: the page reads the sheet again. */
  readonly changed: () => void;
}

/** Opens "Sintonização" (the master's: "Sintonização · Tor"): a bottom sheet on a phone, a 480 px dialog from a tablet up. */
export function openAttunementSheet(
  dialog: MatDialog,
  bottomSheet: MatBottomSheet,
  data: AttunementSheetData,
) {
  return openSheet<AttunementSheet, AttunementSheetData, void>(
    dialog,
    bottomSheet,
    AttunementSheet,
    {
      data,
      ariaLabel: data.isMaster ? `Sintonização · ${data.characterName}` : 'Sintonização',
      labelledBy: 'sheet-t',
      width: '480px',
    },
  );
}

/** The sheet "Mudar sintonização": the section, in the container the width picks. Each tap in it is saved at once. */
@Component({
  selector: 'app-attunement-sheet',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [SheetFrame, AttunementSection],
  template: `
    <app-sheet-frame
      [title]="data.isMaster ? 'Sintonização · ' + data.characterName : 'Sintonização'"
      titleId="sheet-t"
      [phone]="inSheet"
      (closed)="sheet.close()"
    >
      <app-attunement-section
        [attunement]="data.attunement()"
        [viewer]="data.isMaster ? 'master' : 'owner'"
        [characterName]="data.characterName"
        [windowOpen]="data.windowOpen()"
        [dead]="data.dead"
        [campaignId]="data.campaignId"
        [characterId]="data.characterId"
        [revision]="data.revision()"
        [bare]="true"
        (changed)="data.changed()"
      />
    </app-sheet-frame>
  `,
})
export class AttunementSheet {
  protected readonly sheet = injectSheet<AttunementSheetData, void>();
  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;
}
