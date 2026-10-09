import {
  type Encounter,
  ReactionKind,
  type ReactionWindow,
  CombatantKind,
} from '../../../gen/meurpg/play/v1/combat_pb';
import type { SlotChoice } from '../../../gen/meurpg/rules/v1/rules_pb';
import { joinDots } from '../format/text';
import { metersText } from '../units';
import { circleLabel } from './combat-grid';
import { article } from './combat-log';
import { listNames } from './joint-turn';
import { ofThe } from './move-plan';
import { REACTION_NAMES, openWindows } from './reactions';

/**
 * The master's side of the reaction windows (PM-04): every open window, grouped by the action that provoked it, in
 * the order the server answers them, with the numbers that are the master's (`ReactionWindow.trigger`). The server
 * decides the order, who may answer and what each answer does; this only phrases it. A pronoun is never guessed from a
 * name: a player's window reads "pelo jogador". Pure functions, tested without a DOM.
 */

/** The name of the reaction a window offers, as `names_pt.json` writes it. */
export function reactionName(w: ReactionWindow): string {
  switch (w.kind) {
    case ReactionKind.SHIELD:
      return REACTION_NAMES.shield;
    case ReactionKind.UNCANNY_DODGE:
      return REACTION_NAMES.uncannyDodge;
    case ReactionKind.HELLISH_REBUKE:
      return REACTION_NAMES.hellishRebuke;
    case ReactionKind.COUNTERSPELL:
      return REACTION_NAMES.counterspell;
    case ReactionKind.CUTTING_WORDS:
      return REACTION_NAMES.cuttingWords;
    case ReactionKind.DEFLECT_MISSILES:
      return REACTION_NAMES.deflectMissiles;
    case ReactionKind.FEATHER_FALL:
      return REACTION_NAMES.featherFall;
    case ReactionKind.CONCENTRATION_SAVE:
      return 'Teste de concentração';
    case ReactionKind.OPPORTUNITY:
      return 'Ataque de oportunidade';
    default:
      return 'Reação';
  }
}

/** One answer the master gives for an NPC's counterspell: the slot and what it does ("anula sem teste"). */
export interface SlotOption {
  readonly slot: SlotChoice;
  readonly title: string;
  readonly effect: string;
}

/** One row of a queue: a reactor and what it may do. */
export interface QueueRow {
  readonly window: ReactionWindow;
  readonly label: string;
  readonly isPlayer: boolean;
  /** "Jogador" or "NPC". */
  readonly tag: string;
  readonly description: string;
  /** "Respondendo no celular · vez de responder", "Depois do Mago 1", or `''`. */
  readonly status: string;
  readonly answerNow: boolean;
  readonly useLabel: string;
  readonly passLabel: string;
  readonly slots: readonly SlotOption[];
}

/** A card of the master's side. */
export type ReactionCard =
  | {
      readonly type: 'queue';
      readonly id: string;
      readonly title: string;
      readonly subtitle: string;
      readonly rows: readonly QueueRow[];
    }
  | { readonly type: 'single'; readonly id: string; readonly row: QueueRow; readonly text: string }
  | {
      readonly type: 'check';
      readonly id: string;
      readonly window: ReactionWindow;
      readonly summary: string;
      readonly canReact: boolean;
    }
  | { readonly type: 'rebukeSave'; readonly id: string; readonly window: ReactionWindow }
  | {
      readonly type: 'concentration';
      readonly id: string;
      readonly window: ReactionWindow;
      readonly player: boolean;
    };

function theName(label: string): string {
  return `${article(label)} ${label}`;
}

/** "do Mago 1" for an NPC, "de Pensantus" for a player's character: a player's name takes no article. */
function ofReactor(w: ReactionWindow, player: boolean): string {
  return player ? `de ${w.reactorLabel}` : ofThe([w.reactorLabel]);
}

function combatantIsPlayer(e: Encounter, id: string): boolean {
  return e.combatants.some((c) => c.id === id && c.kind === CombatantKind.PLAYER);
}

type Trigger = NonNullable<ReactionWindow['trigger']>;

/** What each reaction adds to its row after the name: the facts of the trigger, in words. */
const ROW_FACTS: {
  readonly [K in ReactionKind]?: (
    w: ReactionWindow,
    t: Trigger | undefined,
    far: string,
  ) => string[];
} = {
  [ReactionKind.COUNTERSPELL]: (_w, _t, far) => [
    'espaço de 3º nível ou maior',
    ...(far ? [far] : []),
  ],
  [ReactionKind.UNCANNY_DODGE]: (w, t) => [
    `o ataque atingiu ${w.reactorLabel}`,
    ...(t ? [`vê ${theName(t.actorLabel)}`] : []),
  ],
  [ReactionKind.SHIELD]: (w) => [`o ataque atingiu ${w.reactorLabel}`],
  [ReactionKind.DEFLECT_MISSILES]: () => ['ataque à distância que acertou'],
  [ReactionKind.HELLISH_REBUKE]: (_w, t) => (t ? [`o dano veio ${ofThe([t.actorLabel])}`] : []),
  [ReactionKind.FEATHER_FALL]: (_w, t) => (t?.actorLabel ? [`${t.actorLabel} cai`] : []),
  [ReactionKind.CUTTING_WORDS]: (_w, t, far) => [
    ...(t ? [`rolagem ${ofThe([t.actorLabel])}`] : []),
    ...(far ? [far] : []),
  ],
};

/** What the window offers, said for the row: "Contramágica · espaço de 3º nível ou maior · a 9 m do Mago 1". */
export function describeRow(w: ReactionWindow): string {
  const t = w.trigger;
  const far =
    t?.distanceFt !== undefined ? `a ${metersText(t.distanceFt)} ${ofThe([t.actorLabel])}` : '';
  const facts = ROW_FACTS[w.kind]?.(w, t, far) ?? (far ? [far] : []);
  return joinDots([reactionName(w), ...facts]);
}

/** What the group's title says: the kind of action that provoked its windows. */
function groupTitle(first: ReactionWindow): string {
  switch (first.kind) {
    case ReactionKind.COUNTERSPELL:
      return 'Reações a uma magia';
    case ReactionKind.FEATHER_FALL:
      return 'Reações a uma queda';
    case ReactionKind.HELLISH_REBUKE:
      return 'Reações a um dano';
    default:
      return 'Reações a um ataque';
  }
}

function groupSubtitle(first: ReactionWindow): string {
  const t = first.trigger;
  if (!t) {
    return 'na ordem da iniciativa';
  }
  switch (first.kind) {
    case ReactionKind.COUNTERSPELL:
      return joinDots([`${t.actionNamePt} ${ofThe([t.actorLabel])}`, 'na ordem da iniciativa']);
    case ReactionKind.SHIELD:
    case ReactionKind.UNCANNY_DODGE:
    case ReactionKind.DEFLECT_MISSILES:
      return joinDots([
        `Ataque ${ofThe([t.actorLabel])}${t.targetLabel ? ` contra ${t.targetLabel}` : ''}`,
        'acertou',
      ]);
    default:
      return joinDots([
        `${t.actionNamePt || reactionName(first)} ${ofThe([t.actorLabel])}`,
        'na ordem da iniciativa',
      ]);
  }
}

/** The slots the master may use for an NPC's Counterspell, with what each does. */
export function slotOptions(w: ReactionWindow): readonly SlotOption[] {
  return (w.trigger?.slotEffects ?? []).flatMap((s) =>
    s.slot
      ? [
          {
            slot: s.slot,
            title: s.slot.pact ? `${circleLabel(s.slot.level)} (pacto)` : circleLabel(s.slot.level),
            effect: s.noCheck ? 'anula sem teste' : `teste de conjuração CD ${s.checkDc}`,
          },
        ]
      : [],
  );
}

function rowOf(e: Encounter, w: ReactionWindow, before: ReactionWindow | undefined): QueueRow {
  const name = reactionName(w);
  const player = w.reactorIsPlayer || combatantIsPlayer(e, w.reactorId);
  const status = player
    ? joinDots(['Respondendo no celular', ...(w.answerNow ? ['vez de responder'] : [])])
    : '';
  return {
    window: w,
    label: w.reactorLabel,
    isPlayer: player,
    tag: player ? 'Jogador' : 'NPC',
    description: describeRow(w),
    status:
      !w.answerNow && before
        ? `Depois ${ofReactor(before, before.reactorIsPlayer || combatantIsPlayer(e, before.reactorId))}`
        : status,
    answerNow: w.answerNow,
    // A player's window is answered "pelo jogador": the name never tells a pronoun.
    useLabel: player
      ? 'Usar pelo jogador'
      : `Usar ${name} ${article(w.reactorLabel) === 'a' ? 'pela' : 'pelo'} ${w.reactorLabel}`,
    passLabel: player ? 'Deixar passar pelo jogador' : 'Deixar passar',
    slots: slotOptions(w),
  };
}

/** The text above an NPC's single card: "Esperando a sua reação: o Mago 1 pode conjurar Escudo Arcano (+5 na CA...)". */
function singleText(e: Encounter, w: ReactionWindow): string {
  const t = w.trigger;
  const who = theName(w.reactorLabel);
  let what: string;
  switch (w.kind) {
    case ReactionKind.SHIELD:
      what =
        t?.armorClassWithShield !== undefined && t.attackTotal !== undefined
          ? `pode conjurar ${REACTION_NAMES.shield} (+5 na CA: ${t.armorClassWithShield} contra ${t.attackTotal} viraria erro)`
          : `pode conjurar ${REACTION_NAMES.shield} (+5 na CA)`;
      break;
    case ReactionKind.COUNTERSPELL:
      what = t?.spellKey
        ? `pode usar ${REACTION_NAMES.counterspell} contra ${t.actionNamePt} (${circleLabel(t.spellLevel)}). Um espaço de ${circleLabel(t.spellLevel)} ou maior a anula sem teste; um espaço menor vale só com o teste`
        : `pode usar ${REACTION_NAMES.counterspell}`;
      break;
    default:
      what = `pode usar ${reactionName(w)}`;
  }
  const reader =
    t && combatantIsPlayer(e, t.actorId)
      ? ` O jogador de ${t.actorLabel} lê só “Esperando o mestre”.`
      : ' Os jogadores leem só “Esperando o mestre”.';
  return `Esperando a sua reação: ${who} ${what}.${reader}`;
}

/** The windows a queue shows: not the checks, the saves, the second steps nor the opportunity attack. */
function inQueue(w: ReactionWindow): boolean {
  return (
    w.kind !== ReactionKind.OPPORTUNITY &&
    w.kind !== ReactionKind.HIDDEN_REVEAL &&
    w.kind !== ReactionKind.MASTER_CHECK &&
    w.kind !== ReactionKind.CONCENTRATION_SAVE &&
    !w.secondStep
  );
}

/** The card of a window that is a card of its own, or `null` for one that belongs to a queue. */
function ownCard(e: Encounter, w: ReactionWindow): ReactionCard | null {
  if (w.kind === ReactionKind.MASTER_CHECK) {
    const p = w.prompt.case === 'masterCheck' ? w.prompt.value : null;
    return {
      type: 'check',
      id: w.id,
      window: w,
      summary: p?.summaryPt ?? '',
      canReact: p?.enemyCanReact ?? false,
    };
  }
  if (w.kind === ReactionKind.CONCENTRATION_SAVE) {
    return {
      type: 'concentration',
      id: w.id,
      window: w,
      player: w.reactorIsPlayer || combatantIsPlayer(e, w.reactorId),
    };
  }
  return w.secondStep && w.prompt.case === 'hellishRebukeSave'
    ? { type: 'rebukeSave', id: w.id, window: w }
    : null;
}

/** The card of one action's windows: an NPC alone is its own card, the others share a queue. */
function groupCard(e: Encounter, key: string, group: readonly ReactionWindow[]): ReactionCard {
  const rows = group.map((x, i) => rowOf(e, x, group[i - 1]));
  if (rows.length === 1 && !rows[0].isPlayer) {
    return { type: 'single', id: key, row: rows[0], text: singleText(e, group[0]) };
  }
  return {
    type: 'queue',
    id: key,
    title: groupTitle(group[0]),
    subtitle: groupSubtitle(group[0]),
    rows,
  };
}

/** The cards of the master's side, in the order the server answers them. */
export function reactionCards(e: Encounter): readonly ReactionCard[] {
  const open = openWindows(e);
  const groups = new Map<string, ReactionWindow[]>();
  for (const w of open.filter(inQueue)) {
    const key = w.groupId || w.id;
    groups.set(key, [...(groups.get(key) ?? []), w]);
  }
  const cards: ReactionCard[] = [];
  const emitted = new Set<string>();
  for (const w of open) {
    const own = ownCard(e, w);
    const key = w.groupId || w.id;
    if (own) {
      cards.push(own);
    } else if (groups.has(key) && !emitted.has(key)) {
      emitted.add(key);
      cards.push(groupCard(e, key, groups.get(key) ?? []));
    }
  }
  return cards;
}

/** Why a held action of the master is off while a window is open: "Espere a reação do Mago 1." or "Responda ao pedido acima.". */
export function heldReason(e: Encounter): string {
  const open = openWindows(e);
  const of = (w: ReactionWindow): string =>
    ofReactor(w, w.reactorIsPlayer || combatantIsPlayer(e, w.reactorId));
  if (open.length === 0) {
    return '';
  }
  const first = open.find((w) => w.answerNow) ?? open[0];
  if (first.kind === ReactionKind.MASTER_CHECK) {
    return 'Responda ao pedido acima.';
  }
  if (first.kind === ReactionKind.CONCENTRATION_SAVE) {
    return `Espere o teste de concentração ${of(first)}.`;
  }
  return first.reactorLabel ? `Espere a reação ${of(first)}.` : 'Responda ao pedido acima.';
}

/** The master's bar while a window is open: "Esperando a sua reação: Mago 1" or who the master waits on. */
export function windowsBarText(e: Encounter): string {
  const open = openWindows(e);
  if (open.length === 0) {
    return '';
  }
  const mine = open.filter(
    (w) => !w.reactorIsPlayer && !combatantIsPlayer(e, w.reactorId) && w.reactorLabel,
  );
  if (mine.length > 0 || open.some((w) => w.kind === ReactionKind.MASTER_CHECK)) {
    return mine.length > 0
      ? `Esperando a sua reação: ${listNames([...new Set(mine.map((w) => w.reactorLabel))])}`
      : 'Esperando a sua resposta';
  }
  return `Esperando a reação de ${listNames([...new Set(open.map((w) => w.reactorLabel))])}`;
}
