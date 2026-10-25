import type { MatDialog } from '@angular/material/dialog';
import type { Observable } from 'rxjs';

import type { Encounter } from '../../../../gen/meurpg/play/v1/combat_pb';
import {
  type StartCombatData,
  StartCombatDialog,
} from '../../../shared/start-combat/start-combat-dialog';

/** The start dialog in its "add" mode (the NPCs only); it closes with the encounter, or with nothing. */
export function openAddCombatants(
  dialog: MatDialog,
  campaignId: string,
  encounter: Encounter,
  phone: boolean,
): Observable<Encounter | undefined> {
  return dialog
    .open<StartCombatDialog, StartCombatData, Encounter>(StartCombatDialog, {
      data: {
        campaignId,
        mode: 'add',
        map: null,
        encounterId: encounter.id,
        existing: encounter.combatants.length,
      },
      width: phone ? '100vw' : '760px',
      maxWidth: phone ? '100vw' : 'calc(100vw - 32px)',
      height: phone ? '100dvh' : undefined,
      maxHeight: phone ? '100dvh' : '92dvh',
      ariaLabelledBy: 'start-title',
      autoFocus: 'first-tabbable',
    })
    .afterClosed();
}
