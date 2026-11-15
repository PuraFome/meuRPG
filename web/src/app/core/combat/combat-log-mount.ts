import { type CombatLogEntry, CombatLogKind } from '../../../gen/meurpg/play/v1/combat_pb';
import type { LogLine } from './combat-log-types';

/** What a builder of these lines is given: the entry and the fields every line shares. */
interface MountArgs {
  readonly e: CombatLogEntry;
  readonly base: { id: string; actor: string; hidden: boolean; undoable: boolean };
}

/** The numbers of a rider's save: "(15 + 2 = 17, CD 10)" for the master and the rider's player, "(falha automática)" for a rider that
 * failed with no d20, and nothing for another player, who is sent no number. */
function mountSaveRoll(e: CombatLogEntry): string {
  const save = e.mountSave;
  if (save?.autoFail) {
    return ' (falha automática)';
  }
  if (!save?.d20) {
    return '';
  }
  const modifier = save.total - save.d20;
  const sign = modifier < 0 ? '-' : '+';
  return ` (${save.d20} ${sign} ${Math.abs(modifier)} = ${save.total}, CD ${save.dc})`;
}

/** The master's note of a fall that found no free square next to the mount. */
function landing(e: CombatLogEntry): string {
  const save = e.mountSave;
  if (!save?.noRoomNextToMount) {
    return '';
  }
  return ` e parou em (${save.landedCol}, ${save.landedRow}), sem lugar ao lado de ${e.targetLabel || 'a montaria'}`;
}

const line = (args: MountArgs, text: string): LogLine => ({ ...args.base, icon: 'pets', text });

/** The lines of a mount (SRD 5.1, "Mounting and Dismounting", "Mounted Combat"). The server sends a line only to who sees both
 * combatants, and the master's alone when either is hidden. A save's numbers are the master's and the rider's player's (RN-10). */
export const MOUNT_LINES: readonly (readonly [CombatLogKind, (args: MountArgs) => LogLine])[] = [
  [CombatLogKind.MOUNTED, (a) => line(a, ` montou em ${a.e.targetLabel || 'uma montaria'}`)],
  [CombatLogKind.DISMOUNTED, (a) => line(a, ` desmontou de ${a.e.targetLabel || 'uma montaria'}`)],
  [CombatLogKind.MOUNT_SAVE, (a) => line(a, ` se segurou na montaria${mountSaveRoll(a.e)}`)],
  [
    CombatLogKind.RIDER_THROWN,
    (a) => line(a, ` caiu da montaria${mountSaveRoll(a.e)}${landing(a.e)}`),
  ],
];
