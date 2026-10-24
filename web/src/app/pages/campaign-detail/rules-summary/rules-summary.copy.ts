import {
  CriticalRule,
  HitPointsRule,
  XpMode,
} from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import type { RulesDraft } from '../../../core/campaigns/table-rules';

/** One line of "Regras da mesa" on the "Regras" tab: a rule and what it is now. */
export interface RuleSummaryRow {
  readonly label: string;
  readonly value: string;
}

const HIT_POINT_WORDS: Readonly<Record<number, string>> = {
  [HitPointsRule.PLAYER_CHOOSES]: 'O jogador escolhe',
  [HitPointsRule.ROLL]: 'Rolar o dado',
  [HitPointsRule.AVERAGE]: 'Média',
};

const CRITICAL_WORDS: Readonly<Record<number, string>> = {
  [CriticalRule.DOUBLED_DICE]: 'Dados dobrados',
  [CriticalRule.MAX_PLUS_ROLL]: 'Máximo dos dados mais uma rolagem',
};

const XP_WORDS: Readonly<Record<number, string>> = {
  [XpMode.ENEMIES]: 'Por inimigos derrotados',
  [XpMode.MILESTONES]: 'Por marcos',
  [XpMode.GOLD]: 'Por ouro encontrado',
};

/** How many ways of making ability scores there are. */
const METHOD_COUNT = 4;

/** The ways of making ability scores a table allows: "Compra de pontos", or "Todos os métodos". */
export function abilityMethodWords(d: RulesDraft): string {
  const methods = [
    { on: d.standardArray, word: 'Conjunto padrão' },
    { on: d.pointBuy, word: 'Compra de pontos' },
    { on: d.rolled4d6, word: '4d6, descartando o menor' },
    { on: d.typed, word: 'Digitar os valores' },
  ].filter((m) => m.on);
  return methods.length === METHOD_COUNT
    ? 'Todos os métodos'
    : methods.map((m) => m.word).join(', ');
}

/** "2 lembretes", "1 lembrete", "Nenhum". */
export function reminderWords(count: number): string {
  if (count === 0) {
    return 'Nenhum';
  }
  return `${count} ${count === 1 ? 'lembrete' : 'lembretes'}`;
}

/** The six rules the tab says in words (board C6-03); the page of the rules says all of them. */
export function ruleSummaryRows(d: RulesDraft, xpMode: XpMode): readonly RuleSummaryRow[] {
  return [
    { label: 'Pontos de vida ao subir de nível', value: HIT_POINT_WORDS[d.hitPoints] ?? '' },
    { label: 'Habilidades de uma ficha nova', value: abilityMethodWords(d) },
    { label: 'Crítico', value: CRITICAL_WORDS[d.critical] ?? '' },
    { label: 'Experiência', value: XP_WORDS[xpMode] ?? '' },
    { label: 'Névoa nos mapas novos', value: d.fogOnNewMaps ? 'Ligada' : 'Desligada' },
    { label: 'Lembretes', value: reminderWords(d.houseRules.length) },
  ];
}
