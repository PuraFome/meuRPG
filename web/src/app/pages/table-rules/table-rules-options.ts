import {
  CriticalRule,
  DeathSaveVisibility,
  DiceMode,
  EnemyReactionsRule,
  HiddenAreaHitRule,
  HitPointsRule,
  TableStyle,
  XpMode,
} from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import type { DiceOption } from '../../core/campaigns/dice-labels';

export const DICE_TITLES: Readonly<Record<number, string>> = {
  [DiceMode.APP]: 'Todos rolam no app',
  [DiceMode.PHYSICAL]: 'Todos rolam os próprios dados',
  [DiceMode.PLAYERS_CHOOSE]: 'Cada jogador escolhe',
};

export const XP_OPTION_TITLES: Readonly<Record<number, string>> = {
  [XpMode.ENEMIES]: 'Por inimigos',
  [XpMode.MILESTONES]: 'Por marcos',
  [XpMode.GOLD]: 'Por ouro',
};

export const STYLE_OPTIONS: readonly DiceOption<TableStyle>[] = [
  {
    value: TableStyle.TUDO_NO_APP,
    title: 'Tudo no app',
    description: 'Dados no app, combate com mapa, névoa ligada nos mapas novos.',
  },
  {
    value: TableStyle.MESA_FISICA,
    title: 'Mesa física',
    description: 'Dados físicos digitados, combate sem mapa por padrão, névoa desligada.',
  },
  {
    value: TableStyle.TEATRO_DA_MENTE,
    title: 'Teatro da mente',
    description: 'Cada jogador escolhe os dados, combate sem mapa, sem névoa.',
  },
  {
    value: TableStyle.PERSONALIZADO,
    title: 'Personalizado',
    description: 'Suas escolhas não batem com nenhum estilo.',
  },
];

export const HIT_POINT_OPTIONS: readonly DiceOption<HitPointsRule>[] = [
  {
    value: HitPointsRule.PLAYER_CHOOSES,
    title: 'O jogador escolhe',
    description: 'Rola o dado ou usa a média, na hora de subir.',
  },
  {
    value: HitPointsRule.ROLL,
    title: 'Rolar o dado',
    description: 'Todos rolam; a média não é oferecida.',
  },
  {
    value: HitPointsRule.AVERAGE,
    title: 'A média',
    description: 'Todos recebem o valor médio do dado de vida.',
  },
];

export const CRITICAL_OPTIONS: readonly DiceOption<CriticalRule>[] = [
  {
    value: CriticalRule.DOUBLED_DICE,
    title: 'Dados dobrados (SRD)',
    description: 'Todos os dados de dano do ataque ou da magia são rolados duas vezes.',
  },
  {
    value: CriticalRule.MAX_PLUS_ROLL,
    title: 'O máximo dos dados mais uma rolagem',
    description: 'Os dados valem o máximo; depois se rola um conjunto.',
  },
];

export const DEATH_SAVE_OPTIONS: readonly DiceOption<DeathSaveVisibility>[] = [
  {
    value: DeathSaveVisibility.VISIBLE_TO_ALL,
    title: 'Todos veem',
    description: 'Os outros jogadores veem as falhas e os sucessos.',
  },
  {
    value: DeathSaveVisibility.OWNER_AND_MASTER,
    title: 'Só o dono e o mestre',
    description: 'Os outros não veem, nem na tela, nem no registro.',
  },
];

export const ENEMY_REACTION_OPTIONS: readonly DiceOption<EnemyReactionsRule>[] = [
  {
    value: EnemyReactionsRule.ONLY_WHEN_POSSIBLE,
    title: 'Só quando um inimigo pode reagir',
    description:
      'O jogo só espera quando algum inimigo tem uma reação disponível. É o padrão e é mais rápido. Atenção: uma pausa pode sugerir aos jogadores que alguém pode reagir.',
  },
  {
    value: EnemyReactionsRule.ALWAYS,
    title: 'Sempre',
    description:
      'Toda ação de um jogador contra um inimigo espera por você, com ou sem reação. Você responde com um toque (“Sem reação”) e a pausa não diz nada aos jogadores. Um pouco mais lento.',
  },
];

/** What an area spell does to the hiding of a hidden creature it hits (PM-02c state 8), from "asks least" to "asks most". */
export const HIDDEN_AREA_OPTIONS: readonly DiceOption<HiddenAreaHitRule>[] = [
  {
    value: HiddenAreaHitRule.REVEAL,
    title: 'Revelar',
    description: 'A criatura aparece para os jogadores, como num “Revelar” seu. É o padrão.',
  },
  {
    value: HiddenAreaHitRule.KEEP_HIDDEN,
    title: 'Manter escondidas',
    description:
      'A criatura sofre o efeito e continua escondida. Os jogadores não ficam sabendo dela.',
  },
  {
    value: HiddenAreaHitRule.ASK,
    title: 'Perguntar a cada vez',
    description:
      'Toda magia de área de um jogador faz o turno esperar, atinja ou não uma criatura escondida (a espera não conta nada a ele); você responde na hora.',
  },
];

/** "Talentos": 1 is allowed, 0 is not used (the radio cards take numbers). */
export const FEAT_OPTIONS: readonly DiceOption<number>[] = [
  {
    value: 0,
    title: 'Não usados',
    description:
      'Todo incremento no valor de habilidade é o do SRD: +2 em uma habilidade ou +1 em duas.',
  },
  {
    value: 1,
    title: 'Permitidos',
    description:
      'No lugar do incremento, o jogador pode pegar um talento da mesa, se a ficha atender ao pré-requisito.',
  },
];

export const DICE_OPTIONS: readonly DiceOption<DiceMode>[] = [
  {
    value: DiceMode.APP,
    title: 'Todos rolam no app',
    description: 'O app rola e registra cada dado.',
  },
  {
    value: DiceMode.PHYSICAL,
    title: 'Todos rolam os próprios dados',
    description: 'Cada jogador digita o que tirou.',
  },
  {
    value: DiceMode.PLAYERS_CHOOSE,
    title: 'Cada jogador escolhe',
    description: 'Rolar no app ou com os próprios dados.',
  },
];

export const COMBAT_OPTIONS: readonly DiceOption<number>[] = [
  { value: 1, title: 'Começar com mapa', description: 'O combate usa o mapa e a grade.' },
  {
    value: 0,
    title: 'Começar sem mapa (teatro da mente)',
    description: 'O mestre julga o alcance; o movimento é por número.',
  },
];
