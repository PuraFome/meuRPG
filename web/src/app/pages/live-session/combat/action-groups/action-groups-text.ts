import type { ActionTabId } from '../../../../core/combat/action-tabs';
import { GRAPPLE_DETAIL, SHOVE_DETAIL } from '../../../../core/combat/contest-view';
import {
  ContestAttackOptionKind,
  type ContestAttackOption,
} from '../../../../../gen/meurpg/play/v1/contest_types_pb';

/** The standard actions an extra action may be, by the key the server sends. */
export const EXTRA_NAMES: Readonly<Record<string, string>> = {
  dash: 'Disparada',
  disengage: 'Desengajar',
  hide: 'Esconder',
  'use-an-object': 'Usar um objeto',
};

/** The icon beside each tab's words (C4-03, C4-04). */
export const TAB_ICONS: Readonly<Record<ActionTabId, string>> = {
  attacks: 'swords',
  spells: 'auto_fix_high',
  class: 'shield',
  other: 'more_horiz',
};

/** "1º": the circle as the slots line writes it. */
export const shortCircle = (level: number): string => `${level}º`;

/** The question Fúria asks a Berserker (SRD 5.1, Path of the Berserker, Frenzy). */
export const FRENZY_QUESTION =
  'Entrar em frenesi? Um ataque corpo a corpo como ação bônus em cada turno seguinte; quando a fúria acabar, você ganha 1 nível de exaustão.';

/** The word of the attack list for Agarrar and Empurrar. */
export function specialAttack(o: ContestAttackOption): { name: string; detail: string } {
  return o.kind === ContestAttackOptionKind.SHOVE
    ? { name: 'Empurrar', detail: SHOVE_DETAIL }
    : { name: 'Agarrar', detail: GRAPPLE_DETAIL };
}
