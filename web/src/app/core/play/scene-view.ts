import { timestampDate } from '@bufbuild/protobuf/wkt';

import type {
  OpenSceneInfo,
  SceneActionView,
  SceneRoll,
} from '../../../gen/meurpg/play/v1/scene_pb';
import { rollFormula } from '../combat/combat-dice';
import { joinDots, tight } from '../format/text';
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
  return roll.passed === undefined ? null : passText(roll.passed, actionOf(scene, roll)?.dc ?? 0);
}

/** "Passou · CD 12" / "Não passou · CD 10"; the word alone for no DC. The DC
 * is tied to its "CD" with a no-break space. */
export function passText(passed: boolean, dc: number): string {
  const word = passed ? 'Passou' : 'Não passou';
  return dc > 0 ? joinDots([word, `CD\u00a0${dc}`]) : word;
}

/** What the master's screen reader hears for a new roll: "Toren: Seguir os
 * rastros dos goblins, 7, não passou". */
export function rollAnnouncement(scene: OpenSceneInfo, roll: SceneRoll): string {
  const parts = [
    `${roll.characterName}: ${rollActionTitle(scene, roll)}`,
    `${roll.roll?.total ?? ''}`,
  ];
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

/** The roll a player made last of an action in this scene, if any (an action
 * with attempts to spare has several; the row keeps the latest result). */
export function ownRollOf(scene: OpenSceneInfo, actionId: string): SceneRoll | undefined {
  let last: SceneRoll | undefined;
  for (const r of scene.rolls) {
    if (r.actionId === actionId && (!last || rolledMs(r) > rolledMs(last))) {
      last = r;
    }
  }
  return last;
}

function rolledMs(roll: SceneRoll): number {
  return roll.rolledAt
    ? Number(roll.rolledAt.seconds) * 1000 + Math.floor(roll.rolledAt.nanos / 1e6)
    : 0;
}

/** What the player's row says about the attempts (MR-015, question 55):
 * "1 tentativa" before rolling, "Restam 2 de 3 tentativas", "Restam N
 * tentativas" when a grant pushed N above the limit, "Sem mais tentativas" at
 * 0, and nothing for an unlimited action (or for no living character). */
export function playerAttempts(
  action: SceneActionView,
): { text: string; out: boolean; left: boolean } | null {
  const left = action.attemptsLeft;
  if (action.maxAttempts === 0 || left === undefined) {
    return null;
  }
  if (left === 0) {
    return { text: 'Sem mais tentativas', out: true, left: false };
  }
  if (left > action.maxAttempts) {
    return { text: tight(`Restam ${left} tentativas`), out: false, left: true };
  }
  if (action.maxAttempts === 1) {
    return { text: '1 tentativa', out: false, left: false };
  }
  return {
    text: tight(`Restam ${left} de ${action.maxAttempts} tentativas`),
    out: false,
    left: true,
  };
}

/** What a player's screen reader hears when an action of the same scene got
 * more attempts than before (the master's grant, or a raised limit); `null`
 * when nothing like that changed. */
export function attemptsAnnouncement(prev: OpenSceneInfo, next: OpenSceneInfo): string | null {
  for (const action of next.actions) {
    const before = prev.actions.find((a) => a.id === action.id)?.attemptsLeft;
    if (before !== undefined && action.attemptsLeft !== undefined && action.attemptsLeft > before) {
      return `O mestre deu mais uma tentativa em ${actionTitle(action)}.`;
    }
  }
  return null;
}

/** The master's "Ações" line: "1 tentativa por jogador", "Sem limite de tentativas". */
export function masterAttempts(action: SceneActionView): string {
  return action.maxAttempts === 0
    ? 'Sem limite de tentativas'
    : `${action.maxAttempts} ${action.maxAttempts === 1 ? 'tentativa' : 'tentativas'} por jogador`;
}

/** The rolls of one character at one action, oldest first. */
function rollsOfCharacter(scene: OpenSceneInfo, roll: SceneRoll): SceneRoll[] {
  return scene.rolls
    .filter((r) => r.actionId === roll.actionId && r.characterId === roll.characterId)
    .sort((a, b) => rolledMs(a) - rolledMs(b));
}

/** The master's roll card says which attempt it was: "Tentativa 1 de 3" (the
 * attempts used plus the ones left), "Tentativa 2" for the roll after a grant
 * pushed it beyond; empty for an unlimited action, which counts nothing. */
export function rollAttemptLine(scene: OpenSceneInfo, roll: SceneRoll): string {
  if (roll.attemptsLeft === undefined) {
    return '';
  }
  const own = rollsOfCharacter(scene, roll);
  const n = own.findIndex((r) => r.id === roll.id) + 1;
  return n === 0 ? '' : `Tentativa ${n} de ${own.length + roll.attemptsLeft}`;
}

/** Whether the roll card offers "Dar mais uma tentativa": the character has
 * none left at this action, this is their last roll of it, and, when the scene
 * shows its DC and the action has one, the roll failed (with the DC hidden, or
 * with no DC to fail, any exhausted roll can be given another try). */
export function canGrantAttempt(scene: OpenSceneInfo, roll: SceneRoll): boolean {
  if (roll.attemptsLeft !== 0) {
    return false;
  }
  const action = actionOf(scene, roll);
  if (!action || action.maxAttempts === 0) {
    return false;
  }
  const own = rollsOfCharacter(scene, roll);
  if (own.at(-1)?.id !== roll.id) {
    return false;
  }
  return !(scene.showDc && action.dc > 0) || roll.passed === false;
}

/** The label of "Dar mais uma tentativa" for a screen reader: who and which action. */
export function grantLabel(scene: OpenSceneInfo, roll: SceneRoll): string {
  return `Dar mais uma tentativa a ${roll.characterName} em ${rollActionTitle(scene, roll)}`;
}

/** "5 ações" / "1 ação". */
export function actionCount(n: number): string {
  return `${n} ${n === 1 ? 'ação' : 'ações'}`;
}

/** "3 rolagens" / "1 rolagem" / "nenhuma rolagem". */
export function rollCount(n: number): string {
  return n === 0 ? 'nenhuma rolagem' : `${n} ${n === 1 ? 'rolagem' : 'rolagens'}`;
}
