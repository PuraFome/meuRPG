import { type CombatLogEntry } from '../../../gen/meurpg/play/v1/combat_pb';
import { castTargetText } from './combat-log-attack';
import { listNames } from './conditions';
import { durationPt } from './creature-action';
import { the } from './combat-log-words';
import type { LogContext } from './combat-log-types';

/**
 * The line of an NPC's action that is no attack roll (`CombatLogKind.SPELL_CAST` with `creature_action`): "usou Sopro de Fogo:
 * Brisa falhou (CD 13), 14 de dano; Toren resistiu (CD 13), 7 de dano. Amedrontado: Brisa, por 1 minuto". The stat block's
 * numbers (the DC, the key) arrive only for the master, so only what arrives is written. An action with no roll is a reminder.
 */
export function creatureActionText(e: CombatLogEntry, ctx: LogContext): string {
  const a = e.creatureAction;
  const name = a?.actionNamePt || e.keyNamePt || 'uma ação';
  if (!a || a.reminder) {
    return ` usou ${name}: lembrete, a ação foi gasta`;
  }
  const targets = e.spell?.targets ?? [];
  let out = ` usou ${name}`;
  if (targets.length > 0) {
    out += `: ${targets.map((t) => castTargetText(t, ctx)).join('; ')}`;
  }
  if (a.conditionNamePt && a.conditionTargetIds.length > 0) {
    const labels = a.conditionTargetIds.map((id) => {
      const label = targets.find((t) => t.targetId === id)?.targetLabel || 'alguém';
      return ctx.players.has(label) ? label : the(label);
    });
    const duration = a.durationText ? `, por ${durationPt(a.durationText)}` : '';
    const repeat = a.repeatSave ? '; repete o teste no fim de cada turno dele' : '';
    out += `. ${a.conditionNamePt}: ${listNames(labels)}${duration}${repeat}`;
  }
  return out;
}
