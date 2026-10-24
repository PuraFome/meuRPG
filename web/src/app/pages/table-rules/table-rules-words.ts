import type { TableStyle, XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import type { DiceOption } from '../../core/campaigns/dice-labels';
import { DICE_WORDS, STYLE_LABELS, type RulesDraft } from '../../core/campaigns/table-rules';
import {
  CRITICAL_OPTIONS,
  DEATH_SAVE_OPTIONS,
  DICE_TITLES,
  ENEMY_REACTION_OPTIONS,
  HIDDEN_AREA_OPTIONS,
  HIT_POINT_OPTIONS,
  XP_OPTION_TITLES,
} from './table-rules-options';

/** What choosing a style did to the choices below it: said in a notice, until the master edits by hand. */
export interface StyleNote {
  readonly title: string;
  readonly changed: readonly string[];
  readonly same: readonly string[];
}

/** The title of the option a rule has chosen. */
function titleOf<T>(options: readonly DiceOption<T>[], value: T): string {
  return options.find((o) => o.value === value)?.title ?? '';
}

/** The ways of making ability scores a table allows, in words. */
function methodWords(d: RulesDraft): string {
  return [
    d.standardArray ? 'Conjunto padrão' : '',
    d.pointBuy ? 'Compra por pontos' : '',
    d.rolled4d6 ? '4d6, descartando o menor' : '',
    d.typed ? 'Digitar os valores' : '',
  ]
    .filter((m) => m !== '')
    .join(', ');
}

/** What a player reads of each rule: its words. */
export function ruleRows(d: RulesDraft, xpMode: XpMode): { label: string; value: string }[] {
  return [
    { label: 'Dados', value: DICE_TITLES[d.diceMode] },
    {
      label: 'Combate',
      value: d.combatStartsWithMap ? 'Começa com mapa' : 'Começa sem mapa (teatro da mente)',
    },
    { label: 'Névoa de guerra nos mapas novos', value: d.fogOnNewMaps ? 'Ligada' : 'Desligada' },
    {
      label: 'Pontos de vida ao subir de nível',
      value: titleOf(HIT_POINT_OPTIONS, d.hitPoints),
    },
    { label: 'Habilidades de uma ficha nova', value: methodWords(d) },
    { label: 'Acertos críticos', value: titleOf(CRITICAL_OPTIONS, d.critical) },
    { label: 'Testes contra a morte', value: titleOf(DEATH_SAVE_OPTIONS, d.deathSaves) },
    {
      label: 'Criaturas escondidas atingidas por uma área',
      value: titleOf(HIDDEN_AREA_OPTIONS, d.hiddenAreaHits),
    },
    { label: 'Talentos', value: d.featsAllowed ? 'Permitidos' : 'Não usados' },
    { label: 'Reações dos inimigos', value: titleOf(ENEMY_REACTION_OPTIONS, d.enemyReactions) },
    {
      label: 'CD dos testes de resistência',
      value: d.showSaveDc ? 'Os jogadores veem' : 'Só o mestre vê',
    },
    { label: 'Experiência', value: XP_OPTION_TITLES[xpMode] ?? '' },
  ];
}

/** What choosing a style did to the three settings it owns: the ones it changed and the ones that were already so. */
export function styleNoteFor(before: RulesDraft, next: RulesDraft, style: TableStyle): StyleNote {
  const changed: string[] = [];
  const same: string[] = [];
  (next.combatStartsWithMap !== before.combatStartsWithMap ? changed : same).push(
    `Combate: ${next.combatStartsWithMap ? 'começar com mapa' : 'começar sem mapa'}.`,
  );
  (next.fogOnNewMaps !== before.fogOnNewMaps ? changed : same).push(
    `Névoa de guerra nos mapas novos: ${next.fogOnNewMaps ? 'ligada' : 'desligada'}.`,
  );
  if (next.diceMode !== before.diceMode) {
    changed.push(`Dados: ${DICE_WORDS[next.diceMode]}.`);
  } else {
    same.push(`Dados: já era “${DICE_TITLES[next.diceMode]}”.`);
  }
  return { title: STYLE_LABELS[style], changed, same };
}

/** "Mudou" and what it was: the three settings the style owns, against what the server has. */
export function wasWords(
  draft: RulesDraft,
  saved: RulesDraft,
): { dice: string; combat: string; fog: string } {
  const onOff = (on: boolean, yes: string, no: string) => (on ? yes : no);
  return {
    dice: draft.diceMode !== saved.diceMode ? DICE_WORDS[saved.diceMode] : '',
    combat:
      draft.combatStartsWithMap !== saved.combatStartsWithMap
        ? onOff(saved.combatStartsWithMap, 'com mapa', 'sem mapa')
        : '',
    fog:
      draft.fogOnNewMaps !== saved.fogOnNewMaps
        ? onOff(saved.fogOnNewMaps, 'ligada', 'desligada')
        : '',
  };
}
