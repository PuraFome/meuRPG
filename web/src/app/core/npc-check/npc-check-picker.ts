import { type NpcCheckCandidate, NpcCheckSource } from '../../../gen/meurpg/play/v1/scene_pb';
import { article } from '../format/article';

/** One creature the master may pick, as the sheet draws its radio row. */
export interface PickRow {
  readonly id: string;
  readonly name: string;
  /** "Os jogadores veem" or "Escondido dos jogadores". */
  readonly sees: string;
  readonly hiddenFromPlayers: boolean;
  /** An NPC typed by hand: its checks roll with +0. */
  readonly noStatBlock: boolean;
  readonly candidate: NpcCheckCandidate;
}

/** The sheet's three groups of "Quem rola". */
export interface PickGroup {
  readonly source: NpcCheckSource;
  readonly label: string;
  readonly icon: string;
  readonly rows: readonly PickRow[];
}

/** One line of the select "Teste": the check and its bonus as the master reads it. */
export interface CheckOptionVm {
  readonly key: string;
  readonly name: string;
  /** "+4", "−1" or "sem bônus". */
  readonly side: string;
  readonly known: boolean;
}

export const SEES_TEXT = 'Os jogadores veem';
export const HIDDEN_TEXT = 'Escondido dos jogadores';
export const BOX_LABEL = 'Os bardos veem a criatura e ela os ouve';
export const NO_STAT_BLOCK_TEXT = 'Sem bloco de estatísticas: rola com +0';
export const WHO_PICK_TEXT = 'Escolha quem rola e o teste.';
export const D20_RANGE_TEXT = 'O d20 vai de 1 a 20.';
/** The check a Cutting Words table most often asks about: the select starts on it when the creature has it. */
const DEFAULT_CHECK = 'skill:deception';
const D20_SIDES = 20;

const GROUPS: readonly { source: NpcCheckSource; label: string; icon: string }[] = [
  { source: NpcCheckSource.STAGE, label: 'Em cena', icon: 'account_circle' },
  { source: NpcCheckSource.TOKEN, label: 'No mapa', icon: 'map' },
  { source: NpcCheckSource.MONSTER, label: 'Do bestiário', icon: 'pets' },
];

/** The creature's id in the picker: its character, else its SRD creature key. */
export function rowId(c: NpcCheckCandidate): string {
  return c.characterId || c.monsterKey;
}

function rowOf(c: NpcCheckCandidate): PickRow {
  return {
    id: rowId(c),
    name: c.name,
    sees: c.hidden ? HIDDEN_TEXT : SEES_TEXT,
    hiddenFromPlayers: c.hidden,
    noStatBlock: !c.hasStatBlock,
    candidate: c,
  };
}

/** The creatures in three groups: the stage and the map only when someone is there, the bestiary always (it is a search). */
export function groupsOf(candidates: readonly NpcCheckCandidate[]): PickGroup[] {
  return GROUPS.map((g) => ({
    ...g,
    rows: candidates.filter((c) => c.source === g.source).map(rowOf),
  })).filter((g) => g.rows.length > 0 || g.source === NpcCheckSource.MONSTER);
}

/** "do Espião", "da Aranha-lobo": the preposition with the creature's article, as the rest of the app writes a creature's name. */
export function ofName(name: string): string {
  return `${article(name) === 'a' ? 'da' : 'do'} ${name}`;
}

/** The line under the select "Teste": whose checks they are. */
export function checkHint(c: NpcCheckCandidate): string {
  return `Uma perícia ou uma habilidade ${ofName(c.name)}, com o bônus ${article(c.name) === 'a' ? 'dela' : 'dele'}.`;
}

/** The box shows only when the map cannot measure the creature against a bard. */
export function showsBox(c: NpcCheckCandidate): boolean {
  return !c.mapMeasures;
}

/** The box starts ticked for a creature the players see and unticked for one the master hid. */
export function boxStart(c: NpcCheckCandidate): boolean {
  return !c.hidden;
}

/** The line under the box, or, when the map measures the creature, the quiet line that stands in its place. */
export function boxHint(c: NpcCheckCandidate): string {
  const the = `${article(c.name) === 'a' ? 'A' : 'O'} ${c.name}`;
  if (c.mapMeasures) {
    return `O token ${ofName(c.name)} e os dos bardos estão no mapa: o app mede os 18 m e a visão. Com o token escondido, nenhum bardo é perguntado.`;
  }
  if (c.hidden) {
    return `${the} está escondido dos jogadores, então a caixa começa desmarcada: ninguém é perguntado. Se você marcar, os bardos leem “uma criatura”, sem o nome.`;
  }
  return `${the} não é medido pelo mapa: é você quem diz. Com o token da criatura e o de um bardo na grade do mapa, o app mede os 18 m e a visão.`;
}

const signed = (n: number) => `${n < 0 ? '−' : '+'}${Math.abs(n)}`;

export function checkOptions(c: NpcCheckCandidate): CheckOptionVm[] {
  return c.checks.map((o) => ({
    key: o.key,
    name: o.name,
    side: o.known ? signed(o.bonus) : 'sem bônus',
    known: o.known,
  }));
}

export function defaultCheck(c: NpcCheckCandidate): string {
  return c.checks.find((o) => o.key === DEFAULT_CHECK)?.key ?? c.checks[0]?.key ?? '';
}

/** Said beside the check when the creature has no number for it: the d20 counts +0 and the master adds the bonus. */
export function noBonusLine(c: NpcCheckCandidate, checkKey: string): string {
  const check = c.checks.find((o) => o.key === checkKey);
  return check && !check.known
    ? `Sem bônus para ${check.name}: o d20 conta com +0 e você soma o bônus que achar justo.`
    : '';
}

/** "Rolar Enganação do Capitão bandido": the button names the roll. */
export function rollLabel(c: NpcCheckCandidate, checkKey: string): string {
  const check = c.checks.find((o) => o.key === checkKey);
  return `Rolar ${check?.name ?? 'o teste'} ${ofName(c.name)}`;
}

/** The d20 the master typed, `null` when it is not a whole number from 1 to 20. */
export function parseD20(text: string): number | null {
  const t = text.trim();
  if (!/^\d{1,2}$/.test(t)) {
    return null;
  }
  const n = Number(t);
  return n >= 1 && n <= D20_SIDES ? n : null;
}
