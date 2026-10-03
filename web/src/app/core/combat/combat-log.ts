import {
  AttackOutcome,
  type CombatLogEntry,
  CombatLogKind,
  type CombatLogRound,
  PendingDamageStatus,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { feetToMeters, formatMeters } from './combat-grid';

/**
 * The combat log as the screens read it (E6-11, E6-14, E6-15): the server
 * sends structured entries, and the sentence is written here, in
 * Portuguese, from what each kind carries. A kind this app doesn't know
 * (a newer server's) is skipped, never shown as an empty line.
 */

/** One line of the log, ready to draw: an icon, the actor in bold, and the
 * rest of the sentence. */
export interface LogLine {
  readonly id: string;
  readonly icon: string;
  /** Who did it (bold); empty for "Combate iniciado". */
  readonly actor: string;
  /** What follows the actor, starting with a space when there is one. */
  readonly text: string;
  /** The master's alone ("Só o mestre vê"). */
  readonly hidden: boolean;
  readonly undoable: boolean;
}

export interface LogGroup {
  readonly round: number;
  /** "Rodada 2". */
  readonly title: string;
  /** "em andamento" or "encerrada". */
  readonly status: string;
  readonly lines: readonly LogLine[];
}

/** "o" or "a" before a name: by its first word's ending, which is right for
 * the names this table uses (Brisa, Toren, Rapieira, Machado, Raio). */
export function article(name: string): 'o' | 'a' {
  const first = name.trim().split(/\s+/)[0].toLowerCase();
  return /a$/.test(first) || /^(foice|rede|clava|mace)$/.test(first) ? 'a' : 'o';
}

/** "no Toren", "na Brisa". */
function inThe(name: string): string {
  return `${article(name) === 'a' ? 'na' : 'no'} ${name}`;
}

/** "o Goblin 1", "a Brisa". */
function the(name: string): string {
  return `${article(name)} ${name}`;
}

/** Whether the attack is made from afar, so the sentence says "atira". */
function isShot(key: string): boolean {
  return /bow|sling|dart|blowgun|net|^spell:/.test(key);
}

function attackText(e: CombatLogEntry): string {
  const target = e.targetLabel || 'alguém';
  const weapon = e.keyNamePt ? ` com ${the(e.keyNamePt)}` : '';
  const verb = isShot(e.key) ? `atira ${inThe(target)}` : `ataca ${the(target)}`;
  let out = ` ${verb}${weapon}: ${e.outcome === AttackOutcome.CRITICAL_HIT ? 'crítico' : e.outcome === AttackOutcome.MISS ? 'errou' : 'acertou'}`;
  if (e.stoppedByReaction) {
    return `${out}, o Escudo segurou`; // the outcome is already "errou"
  }
  const d = e.damage;
  if (d && e.outcome !== AttackOutcome.MISS) {
    switch (d.status) {
      case PendingDamageStatus.AWAITING_ROLL:
        out += ', falta rolar o dano';
        break;
      case PendingDamageStatus.ROLLED:
        out += `, ${d.amount} de dano, esperando o mestre aplicar`;
        break;
      case PendingDamageStatus.DISCARDED:
        out += `, ${d.amount} de dano descartado`;
        break;
      default:
        out += `, ${d.amount} de dano`;
    }
    if (d.targetDefeated) {
      out += `. ${target} derrotado`;
    } else if (d.targetDown) {
      out += `. ${target} caiu`;
    }
  }
  return out;
}

/** What a standard action reads as in the log, by its key. */
function actionText(e: CombatLogEntry): string {
  switch (e.key) {
    case 'standard:hide':
      return ' se esconde';
    case 'standard:search':
      return ' procura ao redor';
    case 'standard:ready':
      return ' se prepara';
    case 'standard:use-an-object':
      return ' usa um objeto';
    default:
      return ` usa ${e.keyNamePt || 'uma ação'}`;
  }
}

function hitPointsText(e: CombatLogEntry): string {
  const n = Math.abs(e.hitPointsDelta);
  const after = e.hitPointsAfter === undefined ? '' : `, agora com ${e.hitPointsAfter} PV`;
  if (e.hitPointsDelta < 0) {
    return ` perdeu ${n} PV, por ajuste do mestre${after}`;
  }
  if (e.hitPointsDelta > 0) {
    return ` recuperou ${n} PV, por ajuste do mestre${after}`;
  }
  return ` teve os PV ajustados pelo mestre${after}`;
}

/** The line of one entry, or `null` for a kind this app doesn't know. */
export function logLine(e: CombatLogEntry, encounterName = ''): LogLine | null {
  const base = { id: e.id, actor: e.actorLabel, hidden: e.hidden, undoable: e.undoable };
  switch (e.kind) {
    case CombatLogKind.COMBAT_BEGUN:
      return { ...base, icon: 'flag', actor: '', text: encounterName ? `Combate iniciado: ${encounterName}` : 'Combate iniciado' };
    case CombatLogKind.ATTACK:
      // A spell attack (Raio de Fogo) has the sparkles E6-15 draws for spells.
      return { ...base, icon: e.key.startsWith('spell:') ? 'auto_awesome' : 'swords', text: attackText(e) };
    case CombatLogKind.ACTION:
      return { ...base, icon: e.key === 'standard:hide' ? 'visibility_off' : 'bolt', text: actionText(e) };
    case CombatLogKind.MOVED:
      return { ...base, icon: 'arrow_forward', text: ` anda ${formatMeters(feetToMeters(e.distanceFt))}` };
    case CombatLogKind.HIT_POINTS_ADJUSTED:
      // The server puts the NPC whose hit points changed in the target.
      return { ...base, actor: e.actorLabel || e.targetLabel, icon: 'healing', text: hitPointsText(e) };
    case CombatLogKind.REVEAL_CHANGED:
      return {
        ...base,
        actor: e.actorLabel || e.targetLabel,
        icon: e.nowHidden ? 'visibility_off' : 'visibility',
        text: e.nowHidden ? ' foi escondido dos jogadores' : ' foi revelado aos jogadores',
      };
    case CombatLogKind.COMBAT_ENDED:
      return { ...base, icon: 'flag', actor: '', text: 'Combate encerrado' };
    default:
      return null;
  }
}

/** The rounds as the log draws them: latest first, the current round
 * "em andamento". Every round after the first ends with "A rodada N
 * começou", the earliest line of its group. */
export function logGroups(rounds: readonly CombatLogRound[], currentRound: number, encounterName = ''): LogGroup[] {
  return rounds.map((r) => {
    const lines = r.entries.map((e) => logLine(e, encounterName)).filter((l): l is LogLine => l !== null);
    if (r.round > 1) {
      lines.push({
        id: `round-${r.round}`,
        icon: 'skip_next',
        actor: '',
        text: `A rodada ${r.round} começou`,
        hidden: false,
        undoable: false,
      });
    }
    return {
      round: r.round,
      title: `Rodada ${r.round}`,
      status: r.round >= currentRound ? 'em andamento' : 'encerrada',
      lines,
    };
  });
}

/** The newest line of all, for the collapsed log. */
export function latestLine(groups: readonly LogGroup[]): LogLine | null {
  for (const g of groups) {
    const first = g.lines.find((l) => !l.id.startsWith('round-'));
    if (first) {
      return first;
    }
  }
  return null;
}

/** How many entries the log has, for "9 entradas" (the synthetic "A rodada
 * começou" lines don't count). */
export function entryCount(groups: readonly LogGroup[]): number {
  return groups.reduce((n, g) => n + g.lines.filter((l) => !l.id.startsWith('round-')).length, 0);
}

/** The entry the undo would take back, named for the confirmation:
 * "o ataque do Capitão Goblin ao Toren (5 de dano)". */
export function undoLabel(e: CombatLogEntry): string {
  switch (e.kind) {
    case CombatLogKind.ATTACK: {
      const target = e.targetLabel || 'alguém';
      const to = article(target) === 'a' ? 'à' : 'ao';
      const damage = e.damage && e.damage.status !== PendingDamageStatus.AWAITING_ROLL && e.outcome !== AttackOutcome.MISS
        ? ` (${e.damage.amount} de dano)`
        : e.outcome === AttackOutcome.MISS
          ? ' (errou)'
          : '';
      return `o ataque ${article(e.actorLabel) === 'a' ? 'da' : 'do'} ${e.actorLabel} ${to} ${target}${damage}`;
    }
    case CombatLogKind.ACTION:
      return `a ação de ${e.actorLabel}${e.keyNamePt ? ` (${e.keyNamePt})` : ''}`;
    case CombatLogKind.HIT_POINTS_ADJUSTED:
      return `o ajuste de PV de ${e.actorLabel || e.targetLabel}`;
    default:
      return 'a última ação';
  }
}

/** The last entry the master may undo, from the groups' source rounds. */
export function undoableEntry(rounds: readonly CombatLogRound[]): CombatLogEntry | null {
  for (const r of rounds) {
    const e = r.entries.find((x) => x.undoable);
    if (e) {
      return e;
    }
  }
  return null;
}

/** The first `max` lines of the log, latest first, keeping the rounds they
 * belong to: the player's panel on a laptop shows a short stretch and
 * "Ver o registro todo" opens the rest. */
export function truncateGroups(groups: readonly LogGroup[], max: number): LogGroup[] {
  const out: LogGroup[] = [];
  let left = max;
  for (const g of groups) {
    if (left <= 0) {
      break;
    }
    const lines = g.lines.slice(0, left);
    left -= lines.length;
    out.push({ ...g, lines });
  }
  return out;
}
