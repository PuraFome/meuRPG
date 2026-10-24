import {
  CriticalRule,
  HitPointsRule,
  XpMode,
} from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { draftFromRules } from '../../../core/campaigns/table-rules';
import { abilityMethodWords, reminderWords, ruleSummaryRows } from './rules-summary.copy';

describe('the rules in words (board C6-03)', () => {
  const base = draftFromRules(undefined);

  it('says the six rules of the tab, in the words of the board', () => {
    const draft = {
      ...base,
      hitPoints: HitPointsRule.AVERAGE,
      standardArray: false,
      pointBuy: true,
      rolled4d6: false,
      typed: false,
      critical: CriticalRule.DOUBLED_DICE,
      fogOnNewMaps: true,
      houseRules: ['a', 'b'],
    };
    expect(ruleSummaryRows(draft, XpMode.ENEMIES)).toEqual([
      { label: 'Pontos de vida ao subir de nível', value: 'Média' },
      { label: 'Habilidades de uma ficha nova', value: 'Compra de pontos' },
      { label: 'Crítico', value: 'Dados dobrados' },
      { label: 'Experiência', value: 'Por inimigos derrotados' },
      { label: 'Névoa nos mapas novos', value: 'Ligada' },
      { label: 'Lembretes', value: '2 lembretes' },
    ]);
  });

  it('names the other choices too', () => {
    const draft = {
      ...base,
      hitPoints: HitPointsRule.ROLL,
      critical: CriticalRule.MAX_PLUS_ROLL,
      fogOnNewMaps: false,
    };
    const rows = ruleSummaryRows(draft, XpMode.MILESTONES);
    expect(rows.map((r) => r.value)).toEqual([
      'Rolar o dado',
      'Todos os métodos',
      'Máximo dos dados mais uma rolagem',
      'Por marcos',
      'Desligada',
      'Nenhum',
    ]);
    expect(ruleSummaryRows(draft, XpMode.GOLD)[3].value).toBe('Por ouro encontrado');
  });

  it('lists the ways to make ability scores when only some are allowed', () => {
    expect(
      abilityMethodWords({
        ...base,
        standardArray: true,
        pointBuy: false,
        rolled4d6: true,
        typed: false,
      }),
    ).toBe('Conjunto padrão, 4d6, descartando o menor');
  });

  it('says one reminder in the singular', () => {
    expect(reminderWords(1)).toBe('1 lembrete');
    expect(reminderWords(0)).toBe('Nenhum');
  });
});
