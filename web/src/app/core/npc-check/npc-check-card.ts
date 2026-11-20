import { NpcCheckAskState } from '../../../gen/meurpg/play/v1/combat_pb';
import { NpcCheckNobodyReason, type NpcCheckRead } from '../../../gen/meurpg/play/v1/scene_pb';
import { initialOf } from '../play/scene-view';

/** How a bard's row reads: the one asked now, the ones behind it, and what each answered. */
export type AskTone = 'waiting' | 'queue' | 'cut' | 'pass' | 'closed';

/** One bard asked, as the master's card draws the row. */
export interface AskRowVm {
  readonly id: string;
  readonly initial: string;
  readonly name: string;
  /** The one word or two of the tag ("Esperando Brisa", "Na fila", "Cortou 4", "Deixou passar", "Sem resposta"). */
  readonly tag: string;
  /** The line under the name ("d8 · a pergunta está aberta"). */
  readonly sub: string;
  readonly tone: AskTone;
}

/** The master's card of an NPC's check, ready to draw. */
export interface NpcCardVm {
  /** What a hidden card is remembered by: the hold, or the roll of a check that settled at once. */
  readonly key: string;
  readonly title: string;
  readonly settled: boolean;
  /** The number in the box: the d20 with its bonus, less what the bards cut. */
  readonly box: number;
  /** "d20 (13) + 4 = 17", or "17 − 4 = 13" once a die came off. */
  readonly formula: string;
  /** The line under the formula. */
  readonly detail: string;
  /** Said when the creature has no known bonus for this check: it rolled +0, and the master adds the bonus by hand. */
  readonly noBonus: string;
  readonly rows: readonly AskRowVm[];
  /** Nobody was asked: the check settled at once. */
  readonly nobody: boolean;
  readonly nobodyText: string;
}

const WAIT_DETAIL =
  'Antes das Palavras de Interrupção: o total fecha quando os bardos responderem.';

/**
 * Why nobody was asked, as the master may read it. The server sends it with the check (`NpcCheckRead.nobody_reason`, a master-only
 * enum); `unknown` is what the card reads when the server names no reason.
 */
export type NobodyReason = 'unknown' | 'no-bard' | 'hidden' | 'immune' | 'unseen';

const NOBODY_TEXTS: Readonly<Record<NobodyReason, string>> = {
  unknown: 'Ninguém foi perguntado: nenhum bardo podia cortar.',
  'no-bard':
    'Ninguém foi perguntado: nenhum bardo tem Palavras de Interrupção com um uso de Inspiração de Bardo.',
  hidden: 'Ninguém foi perguntado: o token da criatura está escondido no mapa.',
  immune: 'Ninguém foi perguntado: a criatura não pode ser enfeitiçada.',
  unseen: 'Ninguém foi perguntado: nenhum bardo vê a criatura.',
};

const NOBODY_REASONS: Readonly<Record<NpcCheckNobodyReason, NobodyReason>> = {
  [NpcCheckNobodyReason.UNSPECIFIED]: 'unknown',
  [NpcCheckNobodyReason.NO_BARD_CAN_CUT]: 'no-bard',
  [NpcCheckNobodyReason.CHARM_IMMUNE]: 'immune',
  [NpcCheckNobodyReason.TOKEN_HIDDEN]: 'hidden',
  [NpcCheckNobodyReason.NOBODY_SEES]: 'unseen',
};

/** The one place that turns the server's `nobody_reason` into the reason the card words. */
export function nobodyReasonOf(read: NpcCheckRead): NobodyReason {
  return NOBODY_REASONS[read.nobodyReason] ?? 'unknown';
}

/** The sentence under a check that settled with nobody asked. */
export function nobodyText(reason: NobodyReason): string {
  return NOBODY_TEXTS[reason];
}

const sign = (n: number) => (n < 0 ? '−' : '+');

/** "d20 (13) + 4": the die and its bonus, the bonus with its sign. */
function d20Text(read: NpcCheckRead): string {
  const roll = read.d20;
  const face = roll?.faces[0] ?? 0;
  const bonus = roll?.modifier ?? 0;
  return `d20 (${face}) ${sign(bonus)} ${Math.abs(bonus)}`;
}

function joinNames(names: readonly string[]): string {
  return names.length < 2
    ? (names[0] ?? '')
    : `${names.slice(0, -1).join(', ')} e ${names[names.length - 1]}`;
}

function rowOf(read: NpcCheckRead, index: number, firstWaiting: number): AskRowVm {
  const a = read.asks[index];
  const die = `d${a.sides}`;
  const base = { id: a.characterId, initial: initialOf(a.characterName), name: a.characterName };
  switch (a.state) {
    case NpcCheckAskState.CUT:
      return { ...base, tag: `Cortou ${a.face}`, sub: `${die} saiu ${a.face}`, tone: 'cut' };
    case NpcCheckAskState.PASSED:
      return { ...base, tag: 'Deixou passar', sub: `${die} · nada gasto`, tone: 'pass' };
    case NpcCheckAskState.CLOSED:
      return {
        ...base,
        tag: 'Sem resposta',
        sub: `${die} · você seguiu sem esperar · nada gasto`,
        tone: 'closed',
      };
    default:
      return index === firstWaiting
        ? { ...base, tag: `Esperando ${a.characterName}`, sub: `${die} · a pergunta está aberta`, tone: 'waiting' }
        : {
            ...base,
            tag: 'Na fila',
            sub: `${die} · depois de ${read.asks[index - 1]?.characterName ?? ''}`.trim(),
            tone: 'queue',
          };
  }
}

/** The card the master reads for the check the server holds or settled (`GetNpcCheck`, `RollNpcCheck`, `SkipNpcCheckAsks`). */
export function cardOf(read: NpcCheckRead, reason: NobodyReason = nobodyReasonOf(read)): NpcCardVm {
  const total = read.d20?.total ?? 0;
  const cutters = read.asks.filter((a) => a.state === NpcCheckAskState.CUT);
  const cutSoFar = cutters.reduce((sum, a) => sum + a.face, 0);
  const box = read.settled ? read.total : total - cutSoFar;
  const firstWaiting = read.asks.findIndex((a) => a.state === NpcCheckAskState.WAITING);
  const rows = read.asks.map((_, i) => rowOf(read, i, firstWaiting));
  let formula = `${d20Text(read)} = ${total}`;
  let detail = read.settled ? 'Ninguém cortou o teste.' : WAIT_DETAIL;
  if (cutters.length > 0) {
    formula = `${total} − ${cutSoFar} = ${box}${read.settled ? '' : ', por enquanto'}`;
    detail = `${d20Text(read)}, menos ${cutters.length === 1 ? 'o corte' : 'os cortes'} de ${joinNames(cutters.map((a) => a.characterName))}`;
  }
  const nobody = read.settled && read.asks.length === 0;
  return {
    key: read.holdId || `${read.creatureLabel}|${read.checkKey}|${read.d20?.faces.join(',')}|${total}`,
    title: `${read.creatureLabel} · ${read.checkName}`,
    settled: read.settled,
    box,
    formula,
    detail: nobody ? '' : detail,
    noBonus: read.modifierKnown
      ? ''
      : `Sem bônus: ${read.creatureLabel} não tem número para ${read.checkName}. O d20 contou com +0: some o bônus que achar justo.`,
    rows,
    nobody,
    nobodyText: nobody ? nobodyText(reason) : '',
  };
}
