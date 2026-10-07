import { Code, ConnectError } from '@connectrpc/connect';

import {
  EncounterBand,
  EncounterBuildBlockedReason,
  EncounterWarning,
} from '../../../gen/meurpg/play/v1/encounters_pb';
import {
  BUGBEAR,
  GOBLIN,
  HOBGOBLIN,
  OGRE,
  buildBlocked,
  evaluation,
  line,
  MIRATHEL,
} from './encounters-testing';
import {
  bandWord,
  barGeometry,
  capLine,
  encounterErrorMessage,
  headline,
  partyByLevel,
  warningLines,
} from './encounter-text';

/** The artboard's encounter (E10-09 state 1): 450 + 400 + 400 + 300 = 1.550 XP for Mirathel. */
const built = () =>
  evaluation([line(OGRE, 1), line(BUGBEAR, 2), line(HOBGOBLIN, 4), line(GOBLIN, 6)]);
/** Over high (state 3): 4 Ogres, 2 Bugbears, 4 Hobgoblins, 6 Goblins = 2.900 XP, 300 over 2.600. */
const over = () =>
  evaluation([line(OGRE, 4), line(BUGBEAR, 2), line(HOBGOBLIN, 4), line(GOBLIN, 6)]);

describe("the bands and the headline of an encounter (MR-043, RN-29): the server's numbers, in words", () => {
  it('writes "Moderada · 1.550 de 1.875 XP" for the artboard\'s encounter', () => {
    const ev = built();
    expect(ev.totalXp).toBe(1550);
    expect(ev.band).toBe(EncounterBand.MODERATE);
    expect(headline(ev).replace(/\s/g, ' ')).toBe('Moderada · 1.550 de 1.875 XP');
  });

  it('writes a low one against the low budget', () => {
    expect(headline(evaluation([line(GOBLIN, 4)])).replace(/\s/g, ' ')).toBe(
      'Baixa · 200 de 1.250 XP',
    );
  });

  it('writes "Acima de alta: 2.900 de 2.600 XP" past the high budget, and no band is ever "mortal"', () => {
    const ev = over();
    expect(ev.band).toBe(EncounterBand.ABOVE_HIGH);
    expect(headline(ev).replace(/\s/g, ' ')).toBe('Acima de alta: 2.900 de 2.600 XP');
    for (const band of [
      EncounterBand.LOW,
      EncounterBand.MODERATE,
      EncounterBand.HIGH,
      EncounterBand.ABOVE_HIGH,
    ]) {
      expect(bandWord(band).toLowerCase()).not.toContain('mortal');
    }
    for (const w of warningLines(ev)) {
      expect(`${w.title} ${w.text}`.toLowerCase()).not.toContain('mortal');
    }
  });

  it("says the warning of an encounter above high with the server's numbers", () => {
    const [w] = warningLines(over());
    expect(w.kind).toBe(EncounterWarning.ABOVE_HIGH);
    expect(w.title).toBe('Passa do orçamento de alta.');
    expect(w.text.replace(/\s/g, ' ')).toBe(
      '2.900 XP contra 2.600 XP: o encontro fica bem acima do que o grupo aguenta. Dá para guardar e jogar assim.',
    );
  });

  it('names the creatures above the cap, an empty party, too many and a gone creature', () => {
    const ev = evaluation([line(OGRE, 1, { aboveCap: true })], {
      warnings: [
        EncounterWarning.ABOVE_CR_CAP,
        EncounterWarning.NO_PARTY,
        EncounterWarning.TOO_MANY,
        EncounterWarning.UNKNOWN_CREATURE,
      ],
    });
    const lines = warningLines(ev);
    expect(lines.map((l) => l.title)).toEqual([
      'Criatura acima do ND máximo.',
      'O grupo está vazio.',
      'Criaturas demais para um combate.',
      'Uma criatura deste encontro não está mais no SRD.',
    ]);
    expect(lines[0].text).toBe('Ogro passa do ND 7, o limite do grupo. Dá para manter.');
  });

  it('says the cap line from the lowest level and the maximum rating', () => {
    expect(capLine(built()).replace(/\s/g, ' ')).toBe(
      'A criatura mais forte pode ter ND 7: o menor nível do grupo (4) mais 3.',
    );
    expect(capLine(evaluation([], { lowestLevel: 0, maxCr: '0' }))).toBe('');
  });

  it('says the party by level in words', () => {
    expect(partyByLevel(MIRATHEL)).toBe('três de nível 4 e um de nível 5');
    expect(partyByLevel(MIRATHEL.slice(0, 1))).toBe('um de nível 4');
  });
});

describe("the bar of the difficulty draws the server's budgets", () => {
  it('puts the three ticks in order and fills the bar to the total', () => {
    const g = barGeometry(built());
    expect(g.low).toBeLessThan(g.moderate);
    expect(g.moderate).toBeLessThan(g.high);
    expect(g.high).toBeLessThan(100);
    expect(g.fill).toBeGreaterThan(g.low);
    expect(g.fill).toBeLessThan(g.moderate + 5);
    expect(g.over).toBe(false);
  });

  it('fills the bar and marks the tip past the high budget', () => {
    const g = barGeometry(over());
    expect(g.fill).toBe(100);
    expect(g.over).toBe(true);
  });
});

describe('the refusals of the builder, by typed detail', () => {
  it('says no party, nothing fits and a creature the SRD lost', () => {
    expect(
      encounterErrorMessage(buildBlocked(EncounterBuildBlockedReason.NO_PARTY), 'generate'),
    ).toContain('O grupo está vazio');
    expect(
      encounterErrorMessage(buildBlocked(EncounterBuildBlockedReason.NOTHING_FITS), 'generate'),
    ).toContain('Nenhuma criatura desse tipo cabe nessa dificuldade');
    expect(
      encounterErrorMessage(buildBlocked(EncounterBuildBlockedReason.UNKNOWN_CREATURE), 'save'),
    ).toContain('não está mais no SRD');
  });

  it("maps the codes and never shows the server's text", () => {
    const invalid = encounterErrorMessage(
      new ConnectError('count out of range', Code.InvalidArgument),
      'evaluate',
    );
    expect(invalid).toContain('Não deu para medir o encontro');
    expect(invalid).not.toContain('count out of range');
    expect(encounterErrorMessage(new ConnectError('x', Code.NotFound), 'save')).toContain(
      'ponto de batalha',
    );
    expect(encounterErrorMessage(new ConnectError('x', Code.NotFound), 'evaluate')).toContain(
      'o mestre dela',
    );
    expect(
      encounterErrorMessage(new ConnectError('x', Code.PermissionDenied), 'evaluate'),
    ).toContain('Só o mestre');
    expect(encounterErrorMessage(new ConnectError('x', Code.Unavailable), 'generate')).toContain(
      'o servidor não respondeu',
    );
  });
});
