import type { Attack, DerivedSheet } from '../../../gen/meurpg/rules/v1/rules_pb';
import type { FullSheet } from '../../../gen/meurpg/characters/v1/characters_pb';
import { type GearVm, type MarksVm, toGearVm } from '../../core/characters/gear';
import type { WeaponMarkVm } from './character-sheet.types';

/** An attack's marks as the engine derived them. */
export function marksOf(a: Pick<Attack, 'silvered' | 'adamantine' | 'magical'>): MarksVm {
  return { silvered: a.silvered, adamantine: a.adamantine, magical: a.magical };
}

/** The marks the sheet itself sets on its carried weapons. */
export function weaponMarksOf(full: FullSheet): WeaponMarkVm[] {
  return full.weaponMarks.map((m) => ({ weaponKey: m.weaponKey, ...marksOf(m) }));
}

/** The gear rows with their weapons' marks: an equipped weapon has them from its attack (the sheet's mark or a table
 * rule, as the engine uses them); a carried one has no attack, so from the sheet's mark. */
export function gearWithMarks(derived: DerivedSheet, full: FullSheet): GearVm[] {
  return derived.gear.map((g) => {
    const vm = toGearVm(g);
    if (vm.kind !== 'weapon') {
      return vm;
    }
    const attack = derived.attacks.find((a) => a.key === g.key);
    const mark = full.weaponMarks.find((m) => m.weaponKey === g.key);
    const source = attack ?? mark;
    return source ? { ...vm, marks: marksOf(source) } : vm;
  });
}
