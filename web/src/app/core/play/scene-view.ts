import { timestampDate } from '@bufbuild/protobuf/wkt';

import type { OpenSceneInfo, SceneActionView, SceneRoll } from '../../../gen/meurpg/play/v1/scene_pb';
import { rollFormula } from '../combat/combat-dice';
import { joinDots } from '../combat/combat-grid';
import { formatClock } from '../../shared/session-time/session-time';
import { actionTitle } from '../maps/scene-actions';

/** A signed bonus as the sheet writes it: "+6", "−1" (a true minus), "+0". */
export function signedBonus(bonus: number): string {
  return bonus < 0 ? `−${Math.abs(bonus)}` : `+${bonus}`;
}

/** The action a roll was made on, if the master has not removed it since. */
export function actionOf(scene: OpenSceneInfo, roll: SceneRoll): SceneActionView | undefined {
  return scene.actions.find((a) => a.id === roll.actionId);
}

/** The title of a roll's action, or the words for one that is gone. */
export function rollActionTitle(scene: OpenSceneInfo, roll: SceneRoll): string {
  const action = actionOf(scene, roll);
  return action ? actionTitle(action) : 'Ação removida da cena';
}

/** The roll's own line, the total included: `1d20 (11) + 6 = 17`, or a typed
 * die `14 + 3 = 17` (no formula at all for a bare face with no bonus). */
export function sceneRollFormula(roll: SceneRoll): string {
  const dice = roll.roll;
  if (!dice) {
    return '';
  }
  return dice.physical && dice.modifier === 0 ? `${dice.total}` : rollFormula(dice);
}

/** "Passou · CD 12", "Não passou · CD 13", or the word alone when the action
 * is gone; `null` when the action had no DC (nothing is shown then). The
 * numbers never split from their words. */
export function passLabel(scene: OpenSceneInfo, roll: SceneRoll): string | null {
  if (roll.passed === undefined) {
    return null;
  }
  const word = roll.passed ? 'Passou' : 'Não passou';
  const dc = actionOf(scene, roll)?.dc ?? 0;
  return dc > 0 ? joinDots([word, `CD ${dc}`]) : word;
}

/** What the master's screen reader hears for a new roll: "Toren: Seguir os
 * rastros dos goblins, 7, não passou". */
export function rollAnnouncement(scene: OpenSceneInfo, roll: SceneRoll): string {
  const parts = [`${roll.characterName}: ${rollActionTitle(scene, roll)}`, `${roll.roll?.total ?? ''}`];
  if (roll.passed !== undefined) {
    parts.push(roll.passed ? 'passou' : 'não passou');
  }
  return parts.join(', ');
}

/** "21:14", local time. */
export function rollClock(roll: SceneRoll): string {
  return roll.rolledAt ? formatClock(timestampDate(roll.rolledAt)) : '';
}

/** The first letter of a name, for the roll's round token. */
export function initialOf(name: string): string {
  return (name.trim().charAt(0) || '?').toLocaleUpperCase('pt-BR');
}

/** The roll a player already made of an action in this scene, if any. */
export function ownRollOf(scene: OpenSceneInfo, actionId: string): SceneRoll | undefined {
  return scene.rolls.find((r) => r.actionId === actionId);
}

/** "5 ações" / "1 ação". */
export function actionCount(n: number): string {
  return `${n} ${n === 1 ? 'ação' : 'ações'}`;
}

/** "3 rolagens" / "1 rolagem" / "nenhuma rolagem". */
export function rollCount(n: number): string {
  return n === 0 ? 'nenhuma rolagem' : `${n} ${n === 1 ? 'rolagem' : 'rolagens'}`;
}
