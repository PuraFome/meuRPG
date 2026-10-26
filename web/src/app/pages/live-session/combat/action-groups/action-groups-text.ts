import type { ActionTabId } from '../../../../core/combat/action-tabs';

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
