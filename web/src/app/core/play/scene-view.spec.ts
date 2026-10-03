import {
  actionCount,
  initialOf,
  ownRollOf,
  passLabel,
  rollAnnouncement,
  rollClock,
  rollCount,
  sceneRollFormula,
  signedBonus,
} from './scene-view';
import { masterScene, playerScene, sceneRoll } from './scene-testing';

describe('scene view', () => {
  const toren = sceneRoll('r1', 'a2', 'Toren', 7, { passed: false });
  const brisa = sceneRoll('r2', 'a4', 'Brisa', 17, {
    roll: { diceCount: 1, diceSides: 20, faces: [14], modifier: 3, total: 17, physical: true },
  });
  const pensantus = sceneRoll('r3', 'a1', 'Pensantus', 17, {
    passed: true,
    roll: { diceCount: 1, diceSides: 20, faces: [11], modifier: 6, total: 17 },
  });
  const scene = masterScene([toren, brisa, pensantus]);

  it('writes the formula the table reads, for the app and for a typed die', () => {
    expect(sceneRollFormula(pensantus)).toBe('1d20 (11) + 6 = 17');
    expect(sceneRollFormula(brisa)).toBe('14 + 3 = 17');
    expect(
      sceneRollFormula(sceneRoll('r', 'a1', 'X', 14, { roll: { diceCount: 1, diceSides: 20, faces: [14], modifier: 0, total: 14, physical: true } })),
    ).toBe('14');
  });

  it('says passed or not with the DC the action has, and nothing without one', () => {
    expect(passLabel(scene, pensantus)?.replace(/ /g, ' ')).toBe('Passou · CD 12');
    expect(passLabel(scene, toren)?.replace(/ /g, ' ')).toBe('Não passou · CD 13');
    // No DC on the action: the server sends no `passed`, and the pill is not drawn.
    expect(passLabel(scene, brisa)).toBeNull();
    // The action was removed since: the word stays, the DC is gone.
    expect(passLabel(scene, sceneRoll('r', 'gone', 'X', 5, { passed: true }))).toBe('Passou');
  });

  it('keeps the number and its CD together', () => {
    expect(passLabel(scene, pensantus)).toContain('CD 12');
  });

  it('reads a roll aloud as "who: action, total, passed"', () => {
    expect(rollAnnouncement(scene, toren)).toBe('Toren: Seguir os rastros dos goblins, 7, não passou');
    expect(rollAnnouncement(scene, pensantus)).toBe('Pensantus: Procurar pistas na carroça, 17, passou');
    expect(rollAnnouncement(scene, brisa)).toBe('Brisa: Percepção, 17');
  });

  it('formats small things', () => {
    expect(signedBonus(6)).toBe('+6');
    expect(signedBonus(0)).toBe('+0');
    expect(signedBonus(-1)).toBe('−1');
    expect(rollClock(toren)).toBe('21:14');
    expect(initialOf('  toren')).toBe('T');
    expect(initialOf('')).toBe('?');
    expect(actionCount(1)).toBe('1 ação');
    expect(actionCount(5)).toBe('5 ações');
    expect(rollCount(0)).toBe('nenhuma rolagem');
    expect(rollCount(1)).toBe('1 rolagem');
    expect(rollCount(3)).toBe('3 rolagens');
  });

  it("finds the player's own roll of an action", () => {
    const mine = playerScene([sceneRoll('r9', 'a1', 'Pensantus', 17)]);
    expect(ownRollOf(mine, 'a1')?.id).toBe('r9');
    expect(ownRollOf(mine, 'a2')).toBeUndefined();
  });
});
