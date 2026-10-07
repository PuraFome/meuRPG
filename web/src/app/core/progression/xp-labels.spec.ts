import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';

import { formatDayAt } from '../../shared/session-time/session-time';
import { XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { XPAwardMode, XPAwardSchema } from '../../../gen/meurpg/progression/v1/progression_pb';
import {
  awardEach,
  awardTotal,
  awardWhen,
  experienceLead,
  givenLine,
  givenText,
  milestoneText,
  modeTag,
  nameList,
  progress,
  undoneLine,
} from './xp-labels';

const nbsp = ' ';

function award(over: Parameters<typeof create<typeof XPAwardSchema>>[1] = {}) {
  return create(XPAwardSchema, {
    id: 'a1',
    mode: XPAwardMode.XP_AWARD_MODE_ENEMIES,
    reason: 'Combate: Emboscada na estrada',
    givenByDisplayName: 'Samuel',
    createdAt: timestampFromDate(new Date(2026, 9, 2, 20, 41)),
    totalXp: 350,
    shares: [
      { characterId: 'p', characterName: 'Pensantus', xp: 116 },
      { characterId: 't', characterName: 'Toren', xp: 116 },
      { characterId: 'b', characterName: 'Brisa', xp: 116 },
    ],
    ...over,
  });
}

describe('the tag of each history line', () => {
  it('names the four modes', () => {
    expect(modeTag(XPAwardMode.XP_AWARD_MODE_ENEMIES)).toBe('Por inimigos');
    expect(modeTag(XPAwardMode.XP_AWARD_MODE_MANUAL)).toBe('Avulso');
    expect(modeTag(XPAwardMode.XP_AWARD_MODE_GOLD)).toBe('Por ouro');
    expect(modeTag(XPAwardMode.XP_AWARD_MODE_MILESTONE)).toBe('Marco');
    expect(modeTag(XPAwardMode.XP_AWARD_MODE_UNSPECIFIED)).toBe('');
  });
});

describe('who got what', () => {
  it('lists names the way a person says them', () => {
    expect(nameList([])).toBe('');
    expect(nameList(['Pensantus'])).toBe('Pensantus');
    expect(nameList(['Pensantus', 'Toren'])).toBe('Pensantus e Toren');
    expect(nameList(['Pensantus', 'Toren', 'Brisa'])).toBe('Pensantus, Toren e Brisa');
  });

  it('says who gave an award and who a milestone', () => {
    expect(givenLine(award())).toBe('Samuel deu a Pensantus, Toren e Brisa');
    expect(givenLine(award({ mode: XPAwardMode.XP_AWARD_MODE_MILESTONE }))).toBe(
      'Samuel marcou Pensantus, Toren e Brisa',
    );
    expect(givenLine(award({ givenByDisplayName: '' }))).toMatch(/^O mestre deu a/);
  });

  it('says how much each one got and the total', () => {
    expect(awardEach(award())).toBe(`116${nbsp}XP para cada`);
    expect(awardTotal(award())).toBe(`Total de${nbsp}350${nbsp}XP`);
  });

  it('writes when, and when it was undone', () => {
    expect(awardWhen(award())).toBe(formatDayAt(new Date(2026, 9, 2, 20, 41)));
    expect(awardWhen(award())).toMatch(/^02\/10\s+às\s+20:41$/);
    // An account that was deleted has no name: "pelo mestre", not "por o mestre".
    expect(undoneLine(award({ undone: true, undoneByDisplayName: '' }))).toBe(
      'Desfeito pelo mestre.',
    );
    expect(
      undoneLine(
        award({
          undone: true,
          undoneByDisplayName: 'Samuel',
          undoneAt: timestampFromDate(new Date(2026, 9, 2, 21, 10)),
        }),
      ),
    ).toBe(`Desfeito por Samuel em ${formatDayAt(new Date(2026, 9, 2, 21, 10))}.`);
  });
});

describe('the progress towards the next level', () => {
  it('says how much is missing', () => {
    const p = progress(2366, 2700, false);
    expect(p.of).toBe(`2.366 de${nbsp}2.700${nbsp}XP`);
    expect(p.missing).toBe(`Faltam${nbsp}334${nbsp}XP`);
    expect(p.percent).toBe(88);
  });

  it('says nothing is missing once it can level up, and caps the bar', () => {
    const p = progress(2716, 2700, true);
    expect(p.missing).toBe('');
    expect(p.percent).toBe(100);
  });

  it('has no next level at 20', () => {
    expect(progress(400000, 0, false)).toEqual({
      of: `400.000${nbsp}XP`,
      percent: 100,
      missing: '',
    });
  });

  it('shows an empty bar for 0 XP', () => {
    expect(progress(0, 300, false)).toEqual({
      of: `0 de${nbsp}300${nbsp}XP`,
      percent: 0,
      missing: `Faltam${nbsp}300${nbsp}XP`,
    });
  });
});

describe('what the master reads after giving', () => {
  it('says the total, each one and what is lost', () => {
    expect(givenText(350, 116, 2)).toBe(
      `350${nbsp}XP dados: 116 para cada. 2${nbsp}XP se perderam na divisão.`,
    );
    expect(givenText(350, 175, 0)).toBe(`350${nbsp}XP dados: 175 para cada.`);
    expect(givenText(100, 33, 1)).toContain(`1${nbsp}XP se perdeu na divisão.`);
  });

  it('says a milestone to everyone, or to some', () => {
    expect(milestoneText(award({ mode: XPAwardMode.XP_AWARD_MODE_MILESTONE }), true)).toBe(
      'Marco registrado: todos podem subir de nível. Pensantus, Toren e Brisa ganharam a marca.',
    );
    expect(milestoneText(award({ shares: [award().shares[0], award().shares[1]] }), false)).toBe(
      'Marco registrado: Pensantus e Toren podem subir de nível.',
    );
  });
});

describe('the line under "Experiência"', () => {
  it('says what the campaign counts, and the next level when the group is at one', () => {
    expect(experienceLead(XpMode.ENEMIES, { level: 4, xp: 2700 })).toBe(
      `XP por inimigos derrotados. O nível 4 pede 2.700${nbsp}XP.`,
    );
    expect(experienceLead(XpMode.GOLD, null)).toBe('XP por ouro encontrado.');
    expect(experienceLead(XpMode.MILESTONES, null)).toMatch(/marcos/);
  });
});
