import {
  AttackOutcome,
  type CombatLogDamage,
  type CombatLogEntry,
  CombatLogKind,
  type CombatLogRound,
  type CombatLogSpellTarget,
  DeathSaveOutcome,
  PendingDamageStatus,
  SaveOutcome,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { rollText } from './combat-dice';
import { conditionName, listNames } from './conditions';
import { feetToMeters, formatMeters } from './combat-grid';
import { circleLabel } from './combat-options';
import { countsSentence } from './death-saves';

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

/** The damage of an attack or of one target of a cast, as the sentence
 * tells it: ", 5 de dano", the dice the master overruled, a heal, a half. */
function damageText(d: CombatLogDamage): string {
  if (d.healing) {
    return ` recupera ${d.amount} PV`;
  }
  switch (d.status) {
    case PendingDamageStatus.AWAITING_ROLL:
      return ', falta rolar o dano';
    case PendingDamageStatus.ROLLED:
      return `, ${d.amount} de dano, esperando o mestre aplicar`;
    case PendingDamageStatus.DISCARDED:
      return `, ${d.amount} de dano descartado`;
    default: {
      // The master may apply another number than the dice made: only he is told both.
      const other = d.rolledAmount !== undefined && d.rolledAmount !== d.amount ? ` (o dado deu ${d.rolledAmount})` : '';
      const half = d.half ? ' (metade)' : '';
      const failures =
        d.deathFailuresAdded > 0
          ? `, ${d.deathFailuresAdded === 1 ? 'uma falha' : 'duas falhas'} no teste contra a morte`
          : '';
      return `, ${d.amount} de dano${half}${other}${failures}`;
    }
  }
}

/** What a hit on a concentrating target reminds the table (RN-22). */
function concentrationText(d: CombatLogDamage | undefined): string {
  return d?.concentrationDc !== undefined
    ? `. Teste de Constituição, CD ${d.concentrationDc}, para manter a concentração`
    : '';
}

function attackText(e: CombatLogEntry): string {
  const target = e.targetLabel || 'alguém';
  const weapon = e.keyNamePt ? ` com ${the(e.keyNamePt)}` : '';
  const verb = isShot(e.key) ? `atira ${inThe(target)}` : `ataca ${the(target)}`;
  const opportunity = e.asReaction ? ' (ataque de oportunidade)' : '';
  let out = ` ${verb}${weapon}${opportunity}: ${e.outcome === AttackOutcome.CRITICAL_HIT ? 'crítico' : e.outcome === AttackOutcome.MISS ? 'errou' : 'acertou'}`;
  if (e.stoppedByReaction) {
    return `${out}, o Escudo Arcano segurou`; // the outcome is already "errou"
  }
  const d = e.damage;
  if (d && e.outcome !== AttackOutcome.MISS) {
    out += damageText(d);
    if (d.targetDefeated) {
      out += `. ${target} derrotado`;
    } else if (d.targetDown) {
      out += `. ${target} caiu`;
    }
    out += concentrationText(d);
  }
  return out;
}

/** One target of a cast: what the roll, the save and the damage did to it. */
function castTargetText(t: CombatLogSpellTarget): string {
  const who = t.targetLabel || 'alguém';
  const damage = t.damage ? damageText(t.damage) : '';
  let out: string;
  if (t.darts > 0) {
    out = `${t.darts} ${t.darts === 1 ? 'dardo' : 'dardos'} ${inThe(who)}${damage}`;
  } else if (t.save) {
    const dc = t.save.dc > 0 ? ` (CD ${t.save.dc})` : '';
    out = `${the(who)} ${t.save.outcome === SaveOutcome.SAVED ? 'resistiu' : 'falhou'}${dc}${damage}`;
  } else if (t.outcome !== AttackOutcome.UNSPECIFIED) {
    out = `${inThe(who)}: ${t.outcome === AttackOutcome.CRITICAL_HIT ? 'crítico' : t.outcome === AttackOutcome.MISS ? 'errou' : 'acertou'}${t.outcome === AttackOutcome.MISS ? '' : damage}`;
  } else if (t.damage?.healing) {
    out = `${the(who)}${damage}`;
  } else {
    out = inThe(who);
  }
  return out;
}

/** "conjura Mísseis Mágicos (1º círculo): 2 dardos no Capitão Goblin, 7 de
 * dano; 1 dardo no Goblin 2, 5 de dano". A spell with no effect the app
 * knows only names who it touches ("conjura Sono no Goblin 1"). */
function castText(e: CombatLogEntry): string {
  const slot = e.spell?.slot;
  const circle = slot ? ` (${circleLabel(slot.level)})` : '';
  const targets = e.spell?.targets ?? [];
  const plain = targets.every((t) => t.darts === 0 && !t.save && t.outcome === AttackOutcome.UNSPECIFIED && !t.damage);
  let out = ` conjura ${e.keyNamePt || 'uma magia'}${circle}`;
  if (targets.length > 0) {
    out += plain ? ` ${listNames(targets.map(castTargetText))}` : `: ${targets.map(castTargetText).join('; ')}`;
  }
  if (e.spell?.concentrationEndedKey) {
    out += '. A concentração anterior acabou';
  }
  return out;
}

function deathSaveText(e: CombatLogEntry): string {
  const s = e.deathSave;
  if (!s) {
    return ' faz um teste contra a morte';
  }
  const word =
    s.outcome === DeathSaveOutcome.REVIVED
      ? 'volta com 1 PV'
      : s.outcome === DeathSaveOutcome.CRITICAL_FAILURE
        ? 'falha crítica, conta duas falhas'
        : s.outcome === DeathSaveOutcome.FAILURE
          ? 'falha'
          : 'sucesso';
  // The d20 is the master's and the character's own player's; the others read the outcome.
  const roll = s.roll ? `rola o teste contra a morte: ${rollText(s.roll)}, ` : 'faz um teste contra a morte: ';
  let out = ` ${roll}${word}`;
  if (s.outcome !== DeathSaveOutcome.REVIVED) {
    out += ` (${countsSentence(s.successes, s.failures)})`;
  }
  if (s.stable) {
    out += '. Estável: não rola mais';
  } else if (s.dying) {
    out += '. Morrendo: o mestre confirma a morte';
  }
  return out;
}

/** "está Envenenado e Derrubado", "ficou sem condições", and the concentration that ended. */
function conditionsText(e: CombatLogEntry): string {
  const parts: string[] = [];
  const names = e.conditions.map(conditionName).filter(Boolean);
  // An entry that only ended a concentration carries no conditions: it must
  // not read as "ficou sem condições".
  if (names.length > 0 || !e.concentrationEndedKey) {
    parts.push(names.length > 0 ? `ficou ${listNames(names)}` : 'ficou sem condições');
  }
  if (e.concentrationEndedKey) {
    parts.push('deixou de se concentrar');
  }
  return ` ${parts.join('; ')}`;
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
export function logLine(
  e: CombatLogEntry,
  encounterName = '',
): LogLine | null {
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
    case CombatLogKind.SPELL_CAST:
      return { ...base, icon: 'auto_awesome', text: castText(e) };
    case CombatLogKind.REACTION: {
      const slot = e.spell?.slot;
      return { ...base, icon: 'shield', text: ` conjura ${e.keyNamePt || 'uma magia'}${slot ? ` (${circleLabel(slot.level)})` : ''}, com a reação` };
    }
    case CombatLogKind.DEATH_SAVE:
      return { ...base, icon: 'heart_broken', text: deathSaveText(e) };
    case CombatLogKind.DEATH_CONFIRMED:
      // The server puts the character in the target.
      return { ...base, actor: e.targetLabel || e.actorLabel, icon: 'close', text: ' morreu' };
    case CombatLogKind.CONDITIONS_CHANGED:
      return { ...base, actor: e.targetLabel || e.actorLabel, icon: 'label', text: conditionsText(e) };
    default:
      return null;
  }
}

/** The rounds as the log draws them: latest first, the current round
 * "em andamento". Every round after the first ends with "A rodada N
 * começou", the earliest line of its group. */
export function logGroups(
  rounds: readonly CombatLogRound[],
  currentRound: number,
  encounterName = '',
): LogGroup[] {
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
    case CombatLogKind.SPELL_CAST:
      return `a magia ${e.keyNamePt || ''} ${article(e.actorLabel) === 'a' ? 'da' : 'do'} ${e.actorLabel}`.replace('  ', ' ');
    case CombatLogKind.REACTION:
      return `a reação ${article(e.actorLabel) === 'a' ? 'da' : 'do'} ${e.actorLabel}`;
    case CombatLogKind.DEATH_SAVE:
      return `o teste contra a morte ${article(e.actorLabel) === 'a' ? 'da' : 'do'} ${e.actorLabel}`;
    case CombatLogKind.CONDITIONS_CHANGED:
      return `a mudança de condições ${article(e.targetLabel) === 'a' ? 'da' : 'do'} ${e.targetLabel}`;
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
