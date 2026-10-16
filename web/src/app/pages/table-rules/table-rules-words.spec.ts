import { DiceMode, TableStyle, XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import type { RulesDraft } from '../../core/campaigns/table-rules';
import { ruleRows, styleNoteFor, wasWords } from './table-rules-words';

describe('the words of the table rules', () => {
  const base: RulesDraft = {
    diceMode: DiceMode.PLAYERS_CHOOSE,
    combatStartsWithMap: true,
    fogOnNewMaps: true,
    hitPoints: 1,
    standardArray: true,
    pointBuy: false,
    rolled4d6: true,
    typed: false,
    critical: 1,
    deathSaves: 1,
    hiddenAreaHits: 1,
    featsAllowed: true,
    enemyReactions: 1,
    houseRules: [],
  } as RulesDraft;

  it('reads each rule of a player in words', () => {
    const rows = ruleRows(base, XpMode.GOLD);
    expect(rows.find((r) => r.label === 'Dados')?.value).toBe('Cada jogador escolhe');
    expect(rows.find((r) => r.label === 'Combate')?.value).toBe('Começa com mapa');
    expect(rows.find((r) => r.label === 'Névoa de guerra nos mapas novos')?.value).toBe('Ligada');
    expect(rows.find((r) => r.label === 'Habilidades de uma ficha nova')?.value).toBe(
      'Conjunto padrão, 4d6, descartando o menor',
    );
    expect(rows.find((r) => r.label === 'Talentos')?.value).toBe('Permitidos');
    expect(rows.find((r) => r.label === 'Experiência')?.value).toBe('Por ouro');
    const other = ruleRows(
      { ...base, combatStartsWithMap: false, fogOnNewMaps: false, featsAllowed: false },
      XpMode.UNSPECIFIED,
    );
    expect(other.find((r) => r.label === 'Combate')?.value).toBe(
      'Começa sem mapa (teatro da mente)',
    );
    expect(other.find((r) => r.label === 'Névoa de guerra nos mapas novos')?.value).toBe(
      'Desligada',
    );
    expect(other.find((r) => r.label === 'Talentos')?.value).toBe('Não usados');
    expect(other.find((r) => r.label === 'Experiência')?.value).toBe('');
  });

  it('splits what a style changed from what was already so', () => {
    const next = { ...base, diceMode: DiceMode.APP, fogOnNewMaps: false };
    const note = styleNoteFor(base, next, TableStyle.TUDO_NO_APP);
    expect(note.changed).toEqual([
      'Névoa de guerra nos mapas novos: desligada.',
      'Dados: todos rolam no app.',
    ]);
    expect(note.same).toEqual(['Combate: começar com mapa.']);
    expect(styleNoteFor(base, base, TableStyle.TUDO_NO_APP).same).toContain(
      'Dados: já era “Cada jogador escolhe”.',
    );
  });

  it('says what each setting was when it changed, and nothing when it did not', () => {
    expect(wasWords(base, base)).toEqual({ dice: '', combat: '', fog: '' });
    const draft = {
      ...base,
      diceMode: DiceMode.APP,
      combatStartsWithMap: false,
      fogOnNewMaps: false,
    };
    expect(wasWords(draft, base)).toEqual({
      dice: 'cada jogador escolhe',
      combat: 'com mapa',
      fog: 'ligada',
    });
    expect(wasWords(base, draft)).toMatchObject({ combat: 'sem mapa', fog: 'desligada' });
  });
});
