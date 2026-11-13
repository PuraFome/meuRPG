import type { CombatLogPowerAttack } from '../../../gen/meurpg/play/v1/combat_pb';
import { signed } from '../effects/effects';

/** What a combat log line says of a power attack: the master and the attacker's player read the numbers
 * ("ataque poderoso (−5 no ataque, +10 no dano)", the signs from the app's `signed`, a true minus); another
 * player reads that a trade was made, or the neutral name the server sent for a feature the master hid
 * ("Outra fonte"). The server sends the numbers (`key` set) to the first two alone (RN-10). */
export function powerAttackWords(p: CombatLogPowerAttack | undefined): string {
  if (!p) {
    return '';
  }
  if (p.key !== '') {
    return `ataque poderoso (${signed(-p.penalty)} no ataque, ${signed(p.damageBonus)} no dano)`;
  }
  return p.hidden ? p.namePt : 'usou um ataque poderoso';
}

/** The words as the attack's sentence takes them: ", com ataque poderoso (−5 no ataque, +10 no dano)". */
export function powerAttackNote(p: CombatLogPowerAttack | undefined): string {
  const words = powerAttackWords(p);
  if (words === '') {
    return '';
  }
  return p?.key !== '' && !p?.hidden ? `, com ${words}` : `, ${words}`;
}
