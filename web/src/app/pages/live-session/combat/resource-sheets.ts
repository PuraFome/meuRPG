import type { MatBottomSheet } from '@angular/material/bottom-sheet';
import type { MatDialog } from '@angular/material/dialog';
import type { Observable } from 'rxjs';

import { openSheet } from '../../../shared/sheet/sheet-host';
import {
  BardicInspirationSheet,
  type BardicInspirationSheetData,
} from './bardic-inspiration-sheet/bardic-inspiration-sheet';
import {
  FlexibleCastingSheet,
  type FlexibleCastingSheetData,
} from './flexible-casting-sheet/flexible-casting-sheet';
import { LayOnHandsSheet, type LayOnHandsSheetData } from './lay-on-hands-sheet/lay-on-hands-sheet';

/** "Cura pelas Mãos": a bottom sheet on a phone, a dialog from a tablet up. Answers `true` when the touch was made. */
export function openLayOnHands(
  dialog: MatDialog,
  bottomSheet: MatBottomSheet,
  data: LayOnHandsSheetData,
): Observable<boolean | undefined> {
  return openSheet<LayOnHandsSheet, LayOnHandsSheetData, boolean>(
    dialog,
    bottomSheet,
    LayOnHandsSheet,
    { data, ariaLabel: 'Cura pelas Mãos', labelledBy: 'sheet-t', width: '480px' },
  );
}

/** "Conjuração Flexível": answers `true` when a slot was created or converted. */
export function openFlexibleCasting(
  dialog: MatDialog,
  bottomSheet: MatBottomSheet,
  data: FlexibleCastingSheetData,
): Observable<boolean | undefined> {
  return openSheet<FlexibleCastingSheet, FlexibleCastingSheetData, boolean>(
    dialog,
    bottomSheet,
    FlexibleCastingSheet,
    { data, ariaLabel: 'Conjuração Flexível', labelledBy: 'sheet-t', width: '480px' },
  );
}

/** "Inspiração de Bardo": answers `true` when the die was given. */
export function openBardicInspiration(
  dialog: MatDialog,
  bottomSheet: MatBottomSheet,
  data: BardicInspirationSheetData,
): Observable<boolean | undefined> {
  return openSheet<BardicInspirationSheet, BardicInspirationSheetData, boolean>(
    dialog,
    bottomSheet,
    BardicInspirationSheet,
    { data, ariaLabel: 'Inspiração de Bardo', labelledBy: 'sheet-t', width: '480px' },
  );
}
