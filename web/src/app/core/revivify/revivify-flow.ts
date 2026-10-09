import {
  type RevivifyRequest,
  type RevivifyTarget,
  RevivifyRequestStatus,
} from '../../../gen/meurpg/play/v1/revivify_pb';
import {
  type CombatLogRound,
  type Encounter,
  EncounterStatus,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { circleLabel } from '../combat/combat-grid';
import { joinDots } from '../format/text';

/** The spell's content key: choosing it in the cast list opens the Revivificar sheet, not the target picker. */
export const REVIVIFY_KEY = 'spell:revivify';

/** What a target the server refuses always says: no reason is ever shown to the player. */
export const REFUSED_TEXT = 'Não dá para reviver esta criatura agora.';
/** What the caster reads when the master says it was more than a minute: nothing else. */
export const DENIED_TEXT = 'O mestre disse que não dá.';
/** Under the dashed "Conjurar Revivificar" until the diamonds are ticked. */
export const DIAMONDS_HINT = 'Marque que você tem os diamantes';
export const DIAMONDS_NOTE =
  'Diamantes de 300 PO. A magia os consome. O app só lembra: quem confere é a mesa.';
export const NOBODY_TEXT = 'Ninguém por perto pode ser revivido agora.';
export const NO_SLOT_TEXT = 'Você não tem espaço de magia livre de 3º\u00a0nível ou maior.';

/** The spell's own circle. */
export const REVIVIFY_LEVEL = 3;

/** True for the spell that brings the dead back. */
export function isRevivify(spellKey: string): boolean {
  return spellKey === REVIVIFY_KEY;
}

export type RevivifyStep = 'target' | 'confirm' | 'result';

export interface StepMark {
  readonly n: number;
  readonly label: string;
  readonly state: 'done' | 'current' | 'todo';
}

const STEP_LABELS: readonly { readonly step: RevivifyStep; readonly label: string }[] = [
  { step: 'target', label: 'Alvo' },
  { step: 'confirm', label: 'Confirmar' },
  { step: 'result', label: 'Resultado' },
];

/** "1 Alvo / 2 Confirmar / 3 Resultado": the ones before the current step are done. */
export function stepMarks(step: RevivifyStep): StepMark[] {
  const at = STEP_LABELS.findIndex((s) => s.step === step);
  return STEP_LABELS.map((s, i) => ({
    n: i + 1,
    label: s.label,
    state: i < at ? 'done' : i === at ? 'current' : 'todo',
  }));
}

export interface TargetRow {
  readonly id: string;
  readonly name: string;
  /** "Ao lado · morreu na rodada 3, há 5 rodadas". */
  readonly detail: string;
  /** "Pode ser revivido" in a combat; empty outside one (the master answers there). */
  readonly tag: string;
  /** Outside a combat: the line that says the master confirms the time. */
  readonly waitsMaster: boolean;
}

/** "há 1 rodada", "há 5 rodadas"; "nesta rodada" when it just happened. */
export function roundsAgo(n: number): string {
  return n <= 0 ? 'nesta rodada' : n === 1 ? 'há 1 rodada' : `há ${n} rodadas`;
}

/** The rows of the list: in a combat the round of the death, outside it only that it was outside. */
export function targetRows(targets: readonly RevivifyTarget[]): TargetRow[] {
  return targets.map((t) => {
    const waitsMaster = t.needsMasterConfirmation;
    const detail = waitsMaster
      ? joinDots(['Ao lado', 'morreu fora de combate'])
      : joinDots([
          'Ao lado',
          t.deathRound === undefined
            ? 'morreu há pouco'
            : `morreu na rodada ${t.deathRound}${
                t.roundsSinceDeath === undefined ? '' : `, ${roundsAgo(t.roundsSinceDeath)}`
              }`,
        ]);
    return {
      id: t.targetId,
      name: t.name,
      detail,
      tag: waitsMaster ? '' : 'Pode ser revivido',
      waitsMaster,
    };
  });
}

/** "3º nível · 1 ação · toque · Ilaria, Clérigo 5". */
export function sheetSubtitle(casterName: string, classes: string): string {
  return joinDots([
    circleLabel(REVIVIFY_LEVEL),
    '1 ação',
    'toque',
    classes ? `${casterName}, ${classes}` : casterName,
  ]);
}

/** "Gasta um espaço de 3º nível (você tem 2) e a sua ação."; outside a combat there is no action. */
export function costLine(level: number, free: number, inCombat: boolean): string {
  const base = `Gasta um espaço de ${circleLabel(level)} (você tem ${free})`;
  return inCombat ? `${base} e a sua ação.` : `${base}.`;
}

/** The cast can be pressed: a target, a slot and the diamonds ticked. */
export function canCast(chosen: string, hasSlot: boolean, diamonds: boolean): boolean {
  return chosen !== '' && hasSlot && diamonds;
}

export interface ResultLines {
  /** "Toren voltou à vida." */
  readonly lead: string;
  /** " Está com 1 PV, acordado e sem testes contra a morte." */
  readonly rest: string;
  /** "Você gastou um espaço de 3º nível (restam 1 de 2) e a sua ação." */
  readonly spent: string;
  readonly diamonds: string;
}

/** What the caster reads once the spell worked; `left` of `had` slots of that level remain. */
export function resultLines(
  name: string,
  level: number,
  left: number,
  had: number,
  inCombat: boolean,
): ResultLines {
  const spent = `Você gastou um espaço de ${circleLabel(level)} (restam ${left} de ${had})`;
  return {
    lead: `${name} voltou à vida.`,
    rest: 'Está com 1 PV, acordado e sem testes contra a morte.',
    spent: inCombat ? `${spent} e a sua ação.` : `${spent}.`,
    diamonds: 'Diamantes de 300 PO gastos, como você confirmou.',
  };
}

export type RequestOutcome = 'pending' | 'confirmed' | 'denied' | 'gone';

/** Where the cast outside a combat stands, from the list the caster reads again. */
export function requestOutcome(
  list: readonly RevivifyRequest[],
  id: string,
): { readonly outcome: RequestOutcome; readonly request?: RevivifyRequest } {
  const request = list.find((r) => r.id === id);
  switch (request?.status) {
    case RevivifyRequestStatus.PENDING:
      return { outcome: 'pending', request };
    case RevivifyRequestStatus.CONFIRMED:
      return { outcome: 'confirmed', request };
    case RevivifyRequestStatus.DENIED:
      return { outcome: 'denied', request };
    default:
      return { outcome: 'gone' };
  }
}

/** Who cast Revivify on this character, from the table's combat log (the newest cast wins); empty when the log does not say. */
export function casterOfRevival(rounds: readonly CombatLogRound[], characterId: string): string {
  const casts = rounds
    .flatMap((r) => r.entries)
    .filter((e) => e.spell?.targets.some((t) => t.effect?.revivedCharacterId === characterId));
  return casts.at(-1)?.actorLabel ?? '';
}

/** Where a revived combatant stands in the order and when it next acts. */
export interface ReturnPlace {
  /** "entre Brisa e Pensantus", or empty when the order does not name two neighbours. */
  readonly between: string;
  readonly round: number;
}

/**
 * The revived character's place, from the combat the player sees: the combatant keeps its place and acts on its next
 * turn, this round if the turn on screen is before it in the order, the next one otherwise. `null` outside a running
 * combat, when the combatant is not in the order yet (the combat is read again) or still down, or when it is its turn.
 */
export function returnPlace(encounter: Encounter | null, characterId: string): ReturnPlace | null {
  if (!encounter || encounter.status !== EncounterStatus.ACTIVE || !characterId) {
    return null;
  }
  const order = encounter.combatants;
  const at = order.findIndex((c) => c.mine && c.characterId === characterId);
  const now = order.findIndex((c) => c.id === encounter.currentCombatantId);
  if (at < 0 || now < 0 || order[at].defeated || at === now) {
    return null;
  }
  const alive = (offset: number) => {
    for (let step = 1; step < order.length; step++) {
      const c = order[(((at + offset * step) % order.length) + order.length) % order.length];
      if (c !== order[at] && !c.defeated) {
        return c;
      }
    }
    return null;
  };
  const before = alive(-1);
  const after = alive(1);
  return {
    between: before && after && before !== after ? `entre ${before.label} e ${after.label}` : '',
    round: at > now ? encounter.round : encounter.round + 1,
  };
}
