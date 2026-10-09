import {
  AttackOutcome,
  type CombatLogDamage,
  type DiceRoll,
  type CombatLogEntry,
  CombatLogKind,
  type CombatLogRound,
  type CombatLogModeChange,
  type CombatLogStateChange,
  type CombatLogSpellTarget,
  DeathSaveOutcome,
  JumpKind,
  PendingDamageStatus,
  type SaveResult,
  SaveOutcome,
  SpellEffectKind,
  SpellEffectOutcome,
  WildShapeEndReason,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { CombatantStateKind, DamageStepKind } from '../../../gen/meurpg/play/v1/combat_rolls_pb';
import { rollText } from './combat-dice';
import { modeWord } from './roll-mode';
import { conditionName, listNames } from './conditions';
import { metersFixed, metersText } from '../units';
import { circleLabel } from './combat-options';
import { countsSentence } from './death-saves';
import { degreeWord, sourceWord } from './cover';
import { effectWords, gainWords, poolRollText, reasonWords } from './hp-effects';
import { trapLogText } from '../traps/trap-log';

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
  /** The master's account of a pool spell (Sono): the dice, each creature and what is left of the total. */
  readonly card?: PoolCard;
  /** The d20 pair of an attack, both faces with the one that counts marked. */
  readonly roll?: LogRoll;
  /** Lines under the sentence: the mode of the roll, the parts of the damage, the resistance steps, a reason. */
  readonly notes?: readonly string[];
}

/** The two d20 of a roll with advantage or disadvantage. */
export interface LogRoll {
  readonly label: string;
  readonly faces: readonly { readonly value: number; readonly counts: boolean }[];
}

/** One creature of the pool card, in the order the pool went through them. */
export interface PoolCardRow {
  readonly id: string;
  readonly label: string;
  /** "7 PV". */
  readonly hitPoints: string;
  /** "15 − 7 = 8 restam", "27 é mais que 8 restantes". */
  readonly math: string;
  /** "Adormeceu · Inconsciente", "Não afetado". */
  readonly word: string;
  readonly icon: string;
  readonly affected: boolean;
}

/** What only the master reads under a Sono or a Leque Cromático (E8-03). */
export interface PoolCard {
  /** "5d8 (2, 4, 1, 5, 3) = 15": the total is the pool of hit points. */
  readonly roll: string;
  /** The caster typed the sum of physical dice: there are no faces, and the card says so. */
  readonly physical: boolean;
  readonly rows: readonly PoolCardRow[];
  /** What the spell does, once, for the one who has not seen it before. */
  readonly note: string;
  /** "Pensantus gastou um espaço de 1º nível.". */
  readonly slot: string;
  /** The creatures that got a condition, to change it ("Mudar as condições do Goblin 1"). */
  readonly changeFor: readonly { readonly id: string; readonly label: string }[];
  /** The same in one sentence, for a screen reader. */
  readonly summary: string;
}

/** Who is reading, for the spells that read hit points: the master's sentence has the hit
 * points and the limit, the player's only who was affected; and a player's character is named
 * without an article ("Brisa"), a creature with one ("o Goblin 1"). */
export interface LogContext {
  readonly master: boolean;
  /** The labels of the player characters in this combat. */
  readonly players: ReadonlySet<string>;
  /** The combat is played without a map (RN-25): a move is "gastou 6,0 m de movimento", and the start says so. */
  readonly theatre?: boolean;
}

const NO_CONTEXT: LogContext = { master: false, players: new Set() };

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
  // "Aranha-lobo gigante" is read by "Aranha", the word the article agrees with.
  const first = name.trim().split(/\s+/)[0].split('-')[0].toLowerCase();
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
      const other =
        d.rolledAmount !== undefined && d.rolledAmount !== d.amount
          ? ` (o dado deu ${d.rolledAmount})`
          : '';
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

/** The master's own sum for an attack on covered target, "(CA 17: 15 + 2 de meia cobertura, do mapa)":
 * only he gets the armor class and the bonus, so nobody else reads it (RN-20). */
function coverNote(
  e: Pick<CombatLogEntry, 'cover' | 'coverSource' | 'targetArmorClass' | 'coverBonus'>,
): string {
  if (e.targetArmorClass === undefined || e.coverBonus <= 0) {
    return '';
  }
  const degree = degreeWord(e.cover).toLowerCase();
  const from = sourceWord(e.coverSource);
  return ` (CA ${e.targetArmorClass}: ${e.targetArmorClass - e.coverBonus} + ${e.coverBonus} de ${degree}${from ? `, ${from.replace('marcada pelo mestre', 'marcada por você')}` : ''})`;
}

function attackText(e: CombatLogEntry): string {
  const target = e.targetLabel || 'alguém';
  const weapon = e.keyNamePt ? ` com ${the(e.keyNamePt)}` : '';
  const verb = isShot(e.key) ? `atira ${inThe(target)}` : `ataca ${the(target)}`;
  const opportunity = e.asReaction ? ' (ataque de oportunidade)' : '';
  let out = ` ${verb}${weapon}${opportunity}: ${e.outcome === AttackOutcome.CRITICAL_HIT ? 'crítico' : e.outcome === AttackOutcome.MISS ? 'errou' : 'acertou'}`;
  out += coverNote(e);
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
  // An opportunity attack that dropped the mover to 0 hit points sends it back to where it left the reach.
  if (e.returnedToReach) {
    out += `. ${target} voltou ao último quadrado dentro do alcance`;
  } else if (e.returnBlocked) {
    out += `. ${target} não pôde voltar ao último quadrado dentro do alcance: ele estava ocupado`;
  }
  return out;
}

/** The parenthesis after a save: the DC, and for the master the d20 of an NPC whose sheet has no
 * saving throw bonus. `bonus_known` is only sent to the master and is false for everyone else, and
 * that false means nothing; for the master it is an NPC whose roll is the bare d20, which the
 * master may overrule. */
function saveNotes(save: SaveResult, ctx: LogContext): string {
  const unknown =
    ctx.master && save.roll && !save.bonusKnown ? `d20 ${save.roll.total}, bônus desconhecido` : '';
  const notes = [save.dc > 0 ? `CD ${save.dc}` : '', unknown].filter((n) => n !== '');
  return notes.length > 0 ? ` (${notes.join('; ')})` : '';
}

/** One target of a cast: what the roll, the save and the damage did to it. */
function castTargetText(t: CombatLogSpellTarget, ctx: LogContext): string {
  const who = t.targetLabel || 'alguém';
  // A spell with two damage types (Tempestade de Gelo) rolls each one: the target's text tells all.
  const damage = t.damage ? [t.damage, ...t.moreDamages].map(damageText).join('') : '';
  let out: string;
  if (t.darts > 0) {
    out = `${t.darts} ${t.darts === 1 ? 'dardo' : 'dardos'} ${inThe(who)}${damage}`;
  } else if (t.save) {
    const dc = saveNotes(t.save, ctx);
    out = `${the(who)} ${t.save.outcome === SaveOutcome.SAVED ? 'resistiu' : 'falhou'}${dc}${damage}`;
  } else if (t.outcome !== AttackOutcome.UNSPECIFIED) {
    out = `${inThe(who)}: ${t.outcome === AttackOutcome.CRITICAL_HIT ? 'crítico' : t.outcome === AttackOutcome.MISS ? 'errou' : 'acertou'}${coverNote(t)}${t.outcome === AttackOutcome.MISS ? '' : damage}`;
  } else if (t.damage?.healing) {
    out = `${the(who)}${damage}`;
  } else {
    out = inThe(who);
  }
  return out;
}

/** "conjura Mísseis Mágicos (1º nível): 2 dardos no Capitão Goblin, 7 de
 * dano; 1 dardo no Goblin 2, 5 de dano". A spell with no effect the app
 * knows only names who it touches ("conjura Sono no Goblin 1"). */
function castText(e: CombatLogEntry, ctx: LogContext): { text: string; card?: PoolCard } {
  if (e.spell && e.spell.effectKind !== SpellEffectKind.UNSPECIFIED) {
    return hpCastText(e, ctx);
  }
  const slot = e.spell?.slot;
  const circle = slot ? ` (${circleLabel(slot.level)})` : '';
  const targets = e.spell?.targets ?? [];
  const plain = targets.every(
    (t) => t.darts === 0 && !t.save && t.outcome === AttackOutcome.UNSPECIFIED && !t.damage,
  );
  let out = ` conjura ${e.keyNamePt || 'uma magia'}${circle}`;
  if (targets.length > 0) {
    out += plain
      ? ` ${listNames(targets.map((t) => castTargetText(t, ctx)))}`
      : `: ${targets.map((t) => castTargetText(t, ctx)).join('; ')}`;
  }
  if (e.spell?.concentrationEndedKey) {
    out += '. A concentração anterior acabou';
  }
  return { text: out };
}

// ---- the spells that read hit points (E8-03) ----

/** "o Goblin 1", "Brisa": a creature has an article, a player's character does not. */
function subject(label: string, ctx: LogContext): string {
  return ctx.players.has(label) ? label : the(label);
}

function upFirst(text: string): string {
  return `${text.charAt(0).toUpperCase()}${text.slice(1)}`;
}

/** "o Goblin 1 adormece. O Capitão Goblin não foi afetado.": one clause for each creature, the
 * first lower case because it follows a colon. */
function clauses(e: CombatLogEntry, ctx: LogContext): string {
  const spell = e.spell!;
  return (spell.targets ?? [])
    .map((t, i) => {
      const label = t.targetLabel || 'alguém';
      const w = effectWords(
        spell.effectKind,
        spell.effectConditionKey,
        t.effect?.outcome ?? SpellEffectOutcome.UNSPECIFIED,
        label,
        t.effect?.gain,
      );
      const verb = w.affected ? w.present : notAffectedPast(label);
      const text = `${subject(label, ctx)} ${verb}.`;
      return i === 0 ? text : upFirst(text);
    })
    .join(' ');
}

function notAffectedPast(label: string): string {
  return effectWords(
    SpellEffectKind.UNSPECIFIED,
    '',
    SpellEffectOutcome.NOT_AFFECTED,
    label,
  ).past.toLowerCase();
}

function hpCastText(e: CombatLogEntry, ctx: LogContext): { text: string; card?: PoolCard } {
  const spell = e.spell!;
  const name = e.keyNamePt || 'uma magia';
  if (!ctx.master) {
    // A player reads who was affected, never why, and never a number of an enemy's (RN-20).
    return { text: ` conjura ${name}: ${clauses(e, ctx)}` };
  }
  if (spell.effectKind === SpellEffectKind.POOL) {
    const card = poolCard(e);
    const slot = spell.slot ? ` (${circleLabel(spell.slot.level)})` : '';
    return { text: ` conjura ${name}${slot}`, card };
  }
  // One creature and its hit points: "no Capitão Goblin (27 PV, limite de 150): fica atordoado."
  const out = (spell.targets ?? []).map((t) => {
    const label = t.targetLabel || 'alguém';
    const fx = t.effect;
    const w = effectWords(
      spell.effectKind,
      spell.effectConditionKey,
      fx?.outcome ?? SpellEffectOutcome.UNSPECIFIED,
      label,
      fx?.gain,
    );
    const prep = ctx.players.has(label) ? `em ${label}` : inThe(label);
    const facts: string[] = [];
    if (fx?.hitPointsBefore !== undefined) {
      facts.push(`${fx.hitPointsBefore} PV`);
    }
    if (spell.effectThreshold !== undefined) {
      facts.push(`limite de ${spell.effectThreshold}`);
    }
    const detail = facts.length > 0 ? ` (${facts.join(', ')})` : '';
    const healed =
      fx?.healed !== undefined && w.affected ? gainWords(spell.effectKind, fx.healed, fx.gain) : '';
    const reason =
      !w.affected && fx
        ? reasonWords(fx.reason, fx.hitPointsBefore, fx.poolLeft, spell.effectThreshold)
        : '';
    const result =
      healed || (w.affected ? w.present : `${w.present}${reason ? `: ${reason}` : ''}`);
    return `${prep}${detail}: ${result}`;
  });
  return { text: ` conjura ${name} ${out.join('; ')}` };
}

/** The master's table for a pool spell: the dice, then each creature from the lowest hit
 * points up with the total that is left, and the word with the condition it got. */
function poolCard(e: CombatLogEntry): PoolCard | undefined {
  const spell = e.spell!;
  if (!spell.poolRoll) {
    return undefined;
  }
  const targets = [...(spell.targets ?? [])].sort((a, b) => {
    const oa = a.effect?.poolOrder ?? Number.MAX_SAFE_INTEGER;
    const ob = b.effect?.poolOrder ?? Number.MAX_SAFE_INTEGER;
    return oa - ob;
  });
  const condition = conditionName(spell.effectConditionKey);
  const rows: PoolCardRow[] = targets.map((t) => {
    const label = t.targetLabel || 'alguém';
    const fx = t.effect;
    const w = effectWords(
      spell.effectKind,
      spell.effectConditionKey,
      fx?.outcome ?? SpellEffectOutcome.UNSPECIFIED,
      label,
      fx?.gain,
    );
    const hp = fx?.hitPointsBefore;
    const left = fx?.poolLeft;
    let math = '';
    if (w.affected && hp !== undefined && left !== undefined) {
      math = `${left + hp} − ${hp} = ${left} restam`;
    } else if (fx && !w.affected) {
      math = reasonWords(fx.reason, hp, left, undefined);
    }
    return {
      id: t.targetId,
      label,
      hitPoints: hp === undefined ? '' : `${hp} PV`,
      math,
      word: w.affected ? `${w.past}${condition ? ` · ${condition}` : ''}` : 'Não afetado',
      icon: w.icon,
      affected: w.affected,
    };
  });
  const roll = poolRollText(spell.poolRoll).replace(' · dado físico', '');
  const summary = `${roll} PV${spell.poolRoll.physical ? ', dado físico' : ''}. ${rows
    .map(
      (r) =>
        `${r.label}${r.hitPoints ? ` (${r.hitPoints})` : ''} ${r.affected ? `${r.word.toLowerCase()}${r.math ? `, ${r.math}` : ''}` : 'não é afetado'}.`,
    )
    .join(' ')}`;
  return {
    roll,
    physical: spell.poolRoll.physical || spell.poolRoll.faces.length === 0,
    rows,
    note: 'A magia começa por quem tem menos PV e vai descontando do total; só afeta quem tem PV igual ou menor que o que sobra. Os PV de quem ficou assim não mudam: a condição não é ferimento.',
    slot: spell.slot ? `${e.actorLabel} gastou um espaço de ${circleLabel(spell.slot.level)}.` : '',
    changeFor: rows
      .filter((r) => r.affected && !!condition)
      .map((r) => ({ id: r.id, label: r.label })),
    summary,
  };
}

function deathSaveText(e: CombatLogEntry): string {
  const s = e.deathSave;
  if (!s) {
    return ' faz um teste contra a morte';
  }
  // A table that hides the death saves (RN-24) tells everyone else only the result: no roll, no outcome, no counts.
  if (s.stable && !s.roll && s.outcome === DeathSaveOutcome.UNSPECIFIED) {
    return ' estabilizou';
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
  const roll = s.roll
    ? `rola o teste contra a morte: ${rollText(s.roll)}, `
    : 'faz um teste contra a morte: ';
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

/** Movement spent by number, in a combat without a map: "gastou 6,0 m de movimento". */
function spentText(e: CombatLogEntry): string {
  const length = e.distanceDft > 0 ? metersFixed(e.distanceDft / 10) : metersFixed(e.distanceFt);
  return ` gastou ${length} de movimento`;
}

/** A move: "anda 3,0 m", "saltou 4,5 m" or "saltou 1,8 m para cima". A long jump that landed in
 * difficult terrain reminds the master (only he gets the flag) of the SRD's Acrobacia check: the app rolls nothing. */
function movedText(e: CombatLogEntry): string {
  if (e.jump === JumpKind.HIGH) {
    return ` saltou ${metersFixed((e.jumpHeightDft || e.distanceDft) / 10)} para cima`;
  }
  const length = e.distanceDft > 0 ? metersFixed(e.distanceDft / 10) : metersText(e.distanceFt);
  if (e.jump === JumpKind.LONG) {
    return ` saltou ${length}${e.landingDifficult ? ' e caiu em terreno difícil. Acrobacia CD 10 ou cai Derrubado' : ''}`;
  }
  return ` anda ${length}`;
}

/** The d20 pair of a roll, or nothing for a single die. */
export function pairOf(roll: DiceRoll | undefined, label: string): LogRoll | undefined {
  if (!roll || roll.diceCount !== 2 || roll.faces.length !== 2) {
    return undefined;
  }
  return {
    label,
    faces: roll.faces.map((value, i) => ({ value, counts: i === roll.countedIndex })),
  };
}

/** "Vantagem (sugerido: Normal) — o alvo está caído": the mode an attack rolled with when it is not what the server suggested. */
function modeNote(m: CombatLogModeChange | undefined): string {
  if (!m) {
    return '';
  }
  const reason = m.reason ? ` — ${m.reason}` : '';
  return `${modeWord(m.mode)} (sugerido: ${modeWord(m.suggestedMode)})${reason}`;
}

/** The dice of each part of a damage, "Espada curta 1d6 (4) + 2 = 6", and the steps resistance took. */
function damageNotes(d: CombatLogDamage | undefined): string[] {
  if (!d) {
    return [];
  }
  const parts = d.parts.map((p) => {
    const dice = p.diceCount > 0 ? `${p.diceCount}d${p.diceSides}` : '';
    const faces = p.faces.length > 0 ? ` (${p.faces.join(', ')})` : '';
    const flat = p.flat !== 0 ? ` ${p.flat > 0 ? '+' : '−'} ${Math.abs(p.flat)}` : '';
    const total = p.sum + p.flat;
    const out = p.counted ? '' : ' — não conta';
    return `${p.labelPt} ${dice}${faces}${dice ? flat : flat.trim()} = ${total}${out}`.trim();
  });
  const steps = d.steps.map((st) => {
    const word =
      st.kind === DamageStepKind.IMMUNITY
        ? 'imunidade'
        : st.kind === DamageStepKind.VULNERABILITY
          ? 'vulnerabilidade'
          : 'resistência';
    const base = `${st.labelPt || word}: ${st.before} → ${st.after}`;
    return st.ignored ? `${base} (o mestre deixou de fora)` : base;
  });
  return [...parts, ...steps];
}

/** "Fúria de Toren começou", "Fúria de Toren acabou (não atacou nem sofreu dano)". */
function stateText(e: CombatLogEntry, st: CombatLogStateChange): string {
  const noun =
    st.kind === CombatantStateKind.RAGE
      ? 'Fúria'
      : st.kind === CombatantStateKind.HUNTERS_MARK_TARGET
        ? 'Marca'
        : st.kind === CombatantStateKind.DODGING
          ? 'Esquiva'
          : st.kind === CombatantStateKind.RECKLESS
            ? 'Ataque descuidado'
            : 'Estado';
  const who = e.actorLabel || e.targetLabel;
  const name = who ? `${noun} de ${who}` : noun;
  if (st.started) {
    return `${name} começou`;
  }
  const reason = stateReason(st.reason);
  return `${name} acabou${reason ? ` (${reason})` : ''}`;
}

function stateReason(key: string): string {
  switch (key) {
    case 'no_attack':
      return 'não atacou nem sofreu dano';
    case 'unconscious':
      return 'ficou inconsciente';
    case 'by_choice':
      return 'por escolha';
    case 'duration':
      return 'a duração acabou';
    case 'left':
      return 'a concentração ou o combate acabou';
    default:
      return key;
  }
}

/** "Brisa pediu Vantagem; o mestre aprovou". */
function modeAnsweredText(m: CombatLogModeChange): string {
  const asked = `pediu ${modeWord(m.mode)}`;
  return ` ${asked}; o mestre ${m.approved ? 'aprovou' : 'recusou'}`;
}

/** The line of one entry, or `null` for a kind this app doesn't know. */
export function logLine(
  e: CombatLogEntry,
  encounterName = '',
  ctx: LogContext = NO_CONTEXT,
): LogLine | null {
  const base = { id: e.id, actor: e.actorLabel, hidden: e.hidden, undoable: e.undoable };
  switch (e.kind) {
    case CombatLogKind.COMBAT_BEGUN:
      return {
        ...base,
        icon: 'flag',
        actor: '',
        text:
          (encounterName ? `Combate iniciado: ${encounterName}` : 'Combate iniciado') +
          (ctx.theatre ? ', sem mapa (teatro da mente)' : ''),
      };
    case CombatLogKind.ATTACK:
      // A spell attack (Raio de Fogo) has the sparkles E6-15 draws for spells.
      return {
        ...base,
        icon: e.key.startsWith('spell:') ? 'auto_awesome' : 'swords',
        text: attackText(e),
        roll: pairOf(e.attackRoll, 'd20 do ataque'),
        notes: [modeNote(e.modeChange), ...damageNotes(e.damage)].filter(Boolean),
      };
    case CombatLogKind.ACTION:
      return {
        ...base,
        icon: e.key === 'standard:hide' ? 'visibility_off' : 'bolt',
        text: actionText(e),
      };
    case CombatLogKind.MOVED:
      return {
        ...base,
        icon: ctx.theatre ? 'directions_run' : 'arrow_forward',
        text: ctx.theatre ? spentText(e) : movedText(e),
      };
    case CombatLogKind.HIT_POINTS_ADJUSTED:
      // The server puts the NPC whose hit points changed in the target.
      return {
        ...base,
        actor: e.actorLabel || e.targetLabel,
        icon: 'healing',
        text: hitPointsText(e),
      };
    case CombatLogKind.REVEAL_CHANGED:
      return {
        ...base,
        actor: e.actorLabel || e.targetLabel,
        icon: e.nowHidden ? 'visibility_off' : 'visibility',
        text: e.nowHidden ? ' foi escondido dos jogadores' : ' foi revelado aos jogadores',
      };
    case CombatLogKind.COMBAT_ENDED:
      return { ...base, icon: 'flag', actor: '', text: 'Combate encerrado' };
    case CombatLogKind.SPELL_CAST: {
      const cast = castText(e, ctx);
      return { ...base, icon: 'auto_awesome', text: cast.text, card: cast.card };
    }
    case CombatLogKind.REACTION: {
      const slot = e.spell?.slot;
      return {
        ...base,
        icon: 'shield',
        text: ` conjura ${e.keyNamePt || 'uma magia'}${slot ? ` (${circleLabel(slot.level)})` : ''}, com a reação`,
      };
    }
    case CombatLogKind.DEATH_SAVE:
      return { ...base, icon: 'heart_broken', text: deathSaveText(e) };
    case CombatLogKind.DEATH_CONFIRMED:
      // The server puts the character in the target.
      return { ...base, actor: e.targetLabel || e.actorLabel, icon: 'close', text: ' morreu' };
    case CombatLogKind.CONDITIONS_CHANGED:
      return {
        ...base,
        actor: e.targetLabel || e.actorLabel,
        icon: 'label',
        text: conditionsText(e),
      };
    case CombatLogKind.TRAP_TRIGGERED:
      // A trap that fired is public: everyone reads the line (the dice stay with the master and the creature's player).
      return { ...base, icon: 'warning', actor: '', text: trapLogText(e.trap, ctx.master) };
    case CombatLogKind.TURN_PART_ENDED:
      // A member of a joint turn ended their part and the turn goes on.
      return {
        ...base,
        icon: 'flag',
        text: ` encerrou a parte ${article(e.actorLabel) === 'a' ? 'dela' : 'dele'}`,
      };
    case CombatLogKind.WILD_SHAPE:
      return { ...base, icon: 'pets', text: wildShapeText(e) };
    case CombatLogKind.OPPORTUNITY_OFFERED:
      // The master offered it, in a combat without a map: the mover is the actor and the reactor the target.
      return { ...base, icon: 'swords', text: ` saiu do alcance de ${e.targetLabel || 'alguém'}` };
    case CombatLogKind.DOOR_OPENED:
      // A move opened a closed door (RN-26): "Toren abriu a porta." The server sends the line only to who saw or remembers the door.
      return { ...base, icon: 'door_open', text: ' abriu a porta' };
    case CombatLogKind.MONSTERS_ADDED:
      // The master put monsters in the combat (RN-29): the line is his alone, with the hit points and the dice they were rolled with.
      return { ...base, hidden: true, icon: 'pets', actor: '', text: monstersAddedText(e) };
    case CombatLogKind.DAMAGE_PART_REMOVED:
      // The master took an extra out of a damage; the reason is only for him and the attacker's player.
      return {
        ...base,
        icon: 'remove_circle_outline',
        actor: '',
        text: `O mestre tirou ${e.keyNamePt ? the(e.keyNamePt) : 'um extra'}${e.reason ? `: “${e.reason}”` : ''}`,
      };
    case CombatLogKind.STATE_CHANGED:
      return e.state
        ? { ...base, icon: 'local_fire_department', actor: '', text: stateText(e, e.state) }
        : null;
    case CombatLogKind.ROLL_MODE_ANSWERED:
      return e.modeChange
        ? {
            ...base,
            icon: 'casino',
            text: modeAnsweredText(e.modeChange),
            notes: e.modeChange.reason ? [`Motivo: “${e.modeChange.reason}”`] : undefined,
          }
        : null;
    default:
      return null;
  }
}

/** "Bandido 1: 9 PV (2d8 + 2: 3, 4)", or "Bandido 1: 11 PV (média)": what the master reads of the monsters that came in. */
export function monstersAddedText(e: CombatLogEntry): string {
  const n = e.monsters.length;
  const each = e.monsters.map((m) => {
    if (!m.rolled) {
      return `${m.label}: ${m.hitPoints} PV (média)`;
    }
    const dice = m.dice.replace(/([+-])/, ' $1 ');
    return `${m.label}: ${m.hitPoints} PV (${dice}: ${m.faces.join(', ')})`;
  });
  return `Você pôs ${n === 1 ? 'um monstro' : `${n} monstros`} no combate. ${each.join('; ')}`;
}

/** "virou o Lobo", "voltou à forma normal": a druid's change of form. The damage that passed on to the druid is only in the
 * entry the master and the druid's player get (RN-20); everyone else reads that the druid is back and why, with no number. */
function wildShapeText(e: CombatLogEntry): string {
  const w = e.wildShape;
  if (!w) {
    return ' mudou de forma';
  }
  if (w.started) {
    return ` virou ${the(w.beastNamePt)}`;
  }
  const she = article(e.actorLabel) === 'a' ? 'ela' : 'ele';
  switch (w.endReason) {
    case WildShapeEndReason.DAMAGE:
      return ` voltou à forma normal: ${the(w.beastNamePt)} caiu a 0 PV${w.carriedDamage > 0 ? ` e ${w.carriedDamage} de dano passaram para ${she}` : ''}`;
    case WildShapeEndReason.MASTER:
      return ` voltou à forma normal, porque o mestre levou ${the(w.beastNamePt)} a 0 PV`;
    case WildShapeEndReason.ZERO_HP:
      return ' voltou à forma normal, ao cair a 0 PV';
    case WildShapeEndReason.UNCONSCIOUS:
      return ' voltou à forma normal, ao ficar inconsciente';
    default:
      return ' voltou à forma normal';
  }
}

/** The rounds as the log draws them: latest first, the current round
 * "em andamento". Every round after the first ends with "A rodada N
 * começou", the earliest line of its group. */
export function logGroups(
  rounds: readonly CombatLogRound[],
  currentRound: number,
  encounterName = '',
  ctx: LogContext = NO_CONTEXT,
): LogGroup[] {
  return rounds.map((r) => {
    const lines = r.entries
      .map((e) => logLine(e, encounterName, ctx))
      .filter((l): l is LogLine => l !== null);
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
      // Round 0 is before the combat begins: the monsters the master put in while it was being set up.
      title: r.round === 0 ? 'Antes do combate' : `Rodada ${r.round}`,
      status:
        r.round === 0
          ? currentRound === 0
            ? 'em preparação'
            : 'encerrada'
          : r.round >= currentRound
            ? 'em andamento'
            : 'encerrada',
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
      const damage =
        e.damage &&
        e.damage.status !== PendingDamageStatus.AWAITING_ROLL &&
        e.outcome !== AttackOutcome.MISS
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
      return `a magia ${e.keyNamePt || ''} ${article(e.actorLabel) === 'a' ? 'da' : 'do'} ${e.actorLabel}`.replace(
        '  ',
        ' ',
      );
    case CombatLogKind.REACTION:
      return `a reação ${article(e.actorLabel) === 'a' ? 'da' : 'do'} ${e.actorLabel}`;
    case CombatLogKind.OPPORTUNITY_OFFERED:
      return `a oferta de ataque de oportunidade a ${e.targetLabel || 'alguém'}`;
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
