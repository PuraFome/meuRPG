import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';

import { PHONE_QUERY } from '../map-view/media-query';
import { SpellDetails, type SpellDetailsData } from './spell-details';

/**
 * Opens the spell's details (the "?"): a bottom sheet on a phone and a 560 px
 * dialog from a tablet up, one component in either (`SpellDetails`). `over` is
 * for the "?" inside another sheet (the cast sheet): a second bottom sheet would
 * close the first and lose its choices, so on a phone it is a dialog laid out
 * like a sheet at the bottom of the screen; "Fechar" goes back to what was under
 * it. Focus returns to the "?" that opened it (Material does it for both).
 */
export function openSpellDetails(
  dialog: MatDialog,
  bottomSheet: MatBottomSheet,
  data: SpellDetailsData,
  over = false,
): void {
  const phone = typeof matchMedia === 'function' && matchMedia(PHONE_QUERY).matches;
  const ariaLabel = `Descrição de ${data.namePt}`;
  if (phone && !over) {
    bottomSheet.open(SpellDetails, { data, ariaLabel, autoFocus: 'first-heading', panelClass: 'mr-sheet' });
  } else if (phone) {
    dialog.open(SpellDetails, {
      data: { ...data, sheet: true },
      ariaLabel,
      autoFocus: 'first-heading',
      width: '100vw',
      maxWidth: '100vw',
      maxHeight: 'calc(100dvh - 24px)',
      position: { bottom: '0' },
      panelClass: 'mr-sheet-over',
    });
  } else {
    dialog.open(SpellDetails, {
      data,
      width: '560px',
      maxWidth: 'calc(100vw - 32px)',
      ariaLabelledBy: 'spell-title',
      autoFocus: 'first-heading',
    });
  }
}
