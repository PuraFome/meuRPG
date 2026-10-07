import {
  actionCount,
  attemptsAnnouncement,
  canGrantAttempt,
  grantLabel,
  masterAttempts,
  playerAttempts,
  rollAttemptLine,
  initialOf,
  ownRollOf,
  passLabel,
  rollAnnouncement,
  rollClock,
  rollCount,
  sceneRollFormula,
  signedBonus,
} from './scene-view';
import { masterScene, playerScene, sceneAction, sceneRoll } from './scene-testing';

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
      sceneRollFormula(
        sceneRoll('r', 'a1', 'X', 14, {
          roll: {
            diceCount: 1,
            diceSides: 20,
            faces: [14],
            modifier: 0,
            total: 14,
            physical: true,
          },
        }),
      ),
    ).toBe('14');
  });

  it('says passed or not with the DC the action has, and nothing without one', () => {
    expect(passLabel(scene, pensantus)?.replace(/\u00a0/g, ' ')).toBe('Passou · CD 12');
    expect(passLabel(scene, toren)?.replace(/\u00a0/g, ' ')).toBe('Não passou · CD 13');
    // No DC on the action: the server sends no `passed`, and the pill is not drawn.
    expect(passLabel(scene, brisa)).toBeNull();
    // The action was removed since: the word stays, the DC is gone.
    expect(passLabel(scene, sceneRoll('r', 'gone', 'X', 5, { passed: true }))).toBe('Passou');
  });

  it('keeps the number and its CD together', () => {
    expect(passLabel(scene, pensantus)).toContain('CD 12');
  });

  it('reads a roll aloud as "who: action, total, passed"', () => {
    expect(rollAnnouncement(scene, toren)).toBe(
      'Toren: Seguir os rastros dos goblins, 7, não passou',
    );
    expect(rollAnnouncement(scene, pensantus)).toBe(
      'Pensantus: Procurar pistas na carroça, 17, passou',
    );
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

  it("finds the player's last roll when an action has several", () => {
    const first = sceneRoll('r1', 'a4', 'Pensantus', 9, { rolledAt: new Date(2026, 9, 3, 21, 15) });
    const second = sceneRoll('r2', 'a4', 'Pensantus', 14, {
      rolledAt: new Date(2026, 9, 3, 21, 17),
    });
    expect(ownRollOf(playerScene([second, first]), 'a4')?.id).toBe('r2');
    expect(ownRollOf(playerScene([first, second]), 'a4')?.id).toBe('r2');
  });

  describe('the attempts (MR-015, question 55)', () => {
    const action = (maxAttempts: number, attemptsLeft?: number) =>
      sceneAction('a', 'Percepção', { maxAttempts, attemptsLeft });
    const text = (maxAttempts: number, attemptsLeft?: number) =>
      playerAttempts(action(maxAttempts, attemptsLeft))?.text.replace(/\u00a0/g, ' ') ?? null;

    it('writes every variant the player reads', () => {
      expect(text(1, 1)).toBe('1 tentativa');
      expect(text(3, 3)).toBe('Restam 3 de 3 tentativas');
      expect(text(3, 2)).toBe('Restam 2 de 3 tentativas');
      expect(text(2, 1)).toBe('Restam 1 de 2 tentativas');
      // A grant pushed it above the limit: no "de M".
      expect(text(1, 2)).toBe('Restam 2 tentativas');
      expect(text(5, 6)).toBe('Restam 6 tentativas');
      expect(text(1, 0)).toBe('Sem mais tentativas');
      expect(playerAttempts(action(1, 0))?.out).toBe(true);
    });

    it('writes nothing for an unlimited action, or when there is no count (no living character)', () => {
      expect(text(0, undefined)).toBeNull();
      expect(text(0, 4)).toBeNull();
      expect(text(3, undefined)).toBeNull();
    });

    it('keeps the number and its words on one line', () => {
      expect(playerAttempts(action(3, 2))?.text).toContain('Restam\u00a02 de\u00a03');
    });

    it("writes the master's line for each action", () => {
      expect(masterAttempts(action(1))).toBe('1 tentativa por jogador');
      expect(masterAttempts(action(3))).toBe('3 tentativas por jogador');
      expect(masterAttempts(action(0))).toBe('Sem limite de tentativas');
    });

    it('announces to a player when an action got more attempts, and only then', () => {
      const before = playerScene();
      const granted = playerScene([], [], {
        actions: before.actions.map((a) => (a.id === 'a5' ? { ...a, attemptsLeft: 2 } : a)),
      });
      expect(attemptsAnnouncement(before, granted)).toBe(
        'O mestre deu mais uma tentativa em Resistir ao cheiro de fumaça.',
      );
      expect(attemptsAnnouncement(granted, before)).toBeNull();
      expect(attemptsAnnouncement(before, before)).toBeNull();
    });
  });

  describe("the master's roll cards", () => {
    const at = (m: number) => new Date(2026, 9, 3, 21, m);
    const exhausted = (id: string, who: string, action: string, m: number, extra = {}) =>
      sceneRoll(id, action, who, 7, { rolledAt: at(m), attemptsLeft: 0, ...extra });

    it('says which attempt it was, out of the ones used and the ones left', () => {
      const one = sceneRoll('r1', 'a4', 'Pensantus', 9, { rolledAt: at(10), attemptsLeft: 2 });
      const two = sceneRoll('r2', 'a4', 'Pensantus', 12, { rolledAt: at(12), attemptsLeft: 2 });
      const other = sceneRoll('r3', 'a4', 'Toren', 8, { rolledAt: at(11), attemptsLeft: 3 });
      const scene = masterScene([two, other, one]);
      expect(rollAttemptLine(scene, one)).toBe('Tentativa 1 de 4');
      expect(rollAttemptLine(scene, two)).toBe('Tentativa 2 de 4');
      expect(rollAttemptLine(scene, other)).toBe('Tentativa 1 de 4');
      // Unlimited: nothing to count (the server leaves attempts_left unset).
      expect(
        rollAttemptLine(
          masterScene([sceneRoll('r4', 'a3', 'Pensantus', 9)]),
          sceneRoll('r4', 'a3', 'Pensantus', 9),
        ),
      ).toBe('');
    });

    it('offers "Dar mais uma tentativa" when the character has none left (the DC off: any roll)', () => {
      const roll = exhausted('r1', 'Pensantus', 'a1', 10, { passed: true });
      expect(canGrantAttempt(masterScene([roll]), roll)).toBe(true);
      const withLeft = sceneRoll('r2', 'a4', 'Pensantus', 9, { attemptsLeft: 2 });
      expect(canGrantAttempt(masterScene([withLeft]), withLeft)).toBe(false);
    });

    it('with the DC shown, offers it only on a failed roll of an action that has a DC', () => {
      const passed = exhausted('r1', 'Pensantus', 'a1', 10, { passed: true });
      const failed = exhausted('r2', 'Toren', 'a2', 11, { passed: false });
      const noDc = exhausted('r3', 'Brisa', 'a4', 12);
      const scene = masterScene([noDc, failed, passed], [], { showDc: true });
      expect(canGrantAttempt(scene, passed)).toBe(false);
      expect(canGrantAttempt(scene, failed)).toBe(true);
      expect(canGrantAttempt(scene, noDc)).toBe(true);
    });

    it('offers it on the last roll only, never on an unlimited action or one that was removed', () => {
      const early = sceneRoll('r1', 'a4', 'Pensantus', 9, { rolledAt: at(10), attemptsLeft: 0 });
      const last = sceneRoll('r2', 'a4', 'Pensantus', 12, { rolledAt: at(12), attemptsLeft: 0 });
      const scene = masterScene([last, early]);
      expect(canGrantAttempt(scene, early)).toBe(false);
      expect(canGrantAttempt(scene, last)).toBe(true);
      const unlimited = exhausted('r5', 'Pensantus', 'a3', 10);
      expect(canGrantAttempt(masterScene([unlimited]), unlimited)).toBe(false);
      const gone = exhausted('r6', 'Pensantus', 'zz', 10);
      expect(canGrantAttempt(masterScene([gone]), gone)).toBe(false);
    });

    it("names the player and the action in the grant's label, so the buttons differ", () => {
      const toren = exhausted('r1', 'Toren', 'a2', 11, { passed: false });
      expect(grantLabel(masterScene([toren]), toren)).toBe(
        'Dar mais uma tentativa a Toren em Seguir os rastros dos goblins',
      );
    });
  });
});
