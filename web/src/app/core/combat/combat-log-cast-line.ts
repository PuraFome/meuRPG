import { type CombatLogEntry, SpellEffectKind } from '../../../gen/meurpg/play/v1/combat_pb';
import { creatureActionText } from './combat-log-creature';
import { castCountedNotes, castTreatedNotes, rerollNote } from './combat-log-dice';
import { castText, reviveCastText } from './combat-log-attack';
import type { LogContext, LogLine } from './combat-log-types';
import { luckyText } from './lucky';

/** The line of a cast (`SPELL_CAST`): a creature's action, a revival or a spell with its targets. */
export function spellCastLine(
  e: CombatLogEntry,
  base: { id: string; actor: string; hidden: boolean; undoable: boolean },
  ctx: LogContext,
): LogLine {
  if (e.creatureAction) {
    // The master also reads the SRD's text of the action, as a line under the sentence.
    return {
      ...base,
      icon: 'bolt',
      text: creatureActionText(e, ctx),
      ...(e.creatureAction.text ? { notes: [`Texto do SRD: ${e.creatureAction.text}`] } : {}),
    };
  }
  if (e.spell?.effectKind === SpellEffectKind.REVIVE) {
    return { ...base, icon: 'favorite', ...reviveCastText(e, ctx) };
  }
  const cast = castText(e, ctx);
  // "Sortudo: 1 → 14": a spell attack's d20 the halfling's Lucky rolled again, or a halfling player's save the app
  // rolled again by itself (the server sends the roll to whoever may read it), kept by the event so a reload still
  // finds it.
  const targets = e.spell?.targets ?? [];
  const lucky = [
    ...targets.flatMap((t) => [rerollNote(t.attackRoll), luckyText(t.save?.roll)]),
    ...castTreatedNotes(targets),
    ...castCountedNotes(targets),
  ].filter(Boolean);
  return {
    ...base,
    icon: 'auto_awesome',
    text: cast.text,
    card: cast.card,
    ...(lucky.length ? { notes: lucky } : {}),
  };
}
