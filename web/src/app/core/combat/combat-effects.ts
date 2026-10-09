/**
 * The two lasting effects that change a combatant's numbers (PM-03a): Escudo Arcano's +5 on the armor class
 * until the start of the caster's next turn, and Ajuda's bonus on the maximum hit points until the master
 * ends it or a long rest. The server sends the numbers (`armor_class_bonus`, `hit_points_max_bonus`) and
 * `hit_points_max` already counts Ajuda; what is written here is the words and the sums of the screens.
 */

/** How long the sentence "O Escudo Arcano acabou" stays on the player's screen. */
export const SHIELD_ENDED_MS = 6000;

/** The armor class on the shield: the sheet's own plus the Escudo Arcano's bonus ("18"). */
export function armorClassWithShield(base: number, bonus: number): number {
  return base + Math.max(0, bonus);
}

/** The small sum under the shield: "13 + 5". */
export function shieldSum(base: number, bonus: number): string {
  return `${base} + ${bonus}`;
}

/** The label under the two cards of a player: "Escudo Arcano +5 até a sua vez". */
export function shieldLabelForOwner(bonus: number): string {
  return `Escudo Arcano +${bonus} até a sua vez`;
}

/** The label on the master's row: "Escudo Arcano +5 até a vez dele". */
export function shieldLabelForMaster(bonus: number): string {
  return `Escudo Arcano +${bonus} até a vez dele`;
}

/** The sentence when the shield ends on the player's own combatant, in the two parts the screen draws (the first
 * in bold): "O Escudo Arcano acabou." and "A sua CA voltou a 13.". */
export function shieldEndedParts(base: number | null): { lead: string; rest: string } {
  return {
    lead: 'O Escudo Arcano acabou.',
    rest: base === null ? '' : `A sua CA voltou a ${base}.`,
  };
}

/** The same sentence in one run, for a screen reader and for the tests. */
export function shieldEndedSentence(base: number | null): string {
  const { lead, rest } = shieldEndedParts(base);
  return rest ? `${lead} ${rest}` : lead;
}

/** The master's list says it in one line: "O Escudo Arcano de Pensantus acabou". */
export function shieldEndedForMaster(label: string): string {
  return `O Escudo Arcano de ${label} acabou`;
}

/** The tag hanging off the maximum of the player's hit points: "+5 de Ajuda". */
export function aidTag(bonus: number): string {
  return `+${bonus} de Ajuda`;
}

/** The master's tag by the maximum: "+5 Ajuda". */
export function aidTagForMaster(bonus: number): string {
  return `+${bonus} Ajuda`;
}

/** The label under the player's cards: "Ajuda: +5 nos PV até o mestre encerrar ou um descanso longo". */
export function aidLabel(bonus: number): string {
  return `Ajuda: +${bonus} nos PV até o mestre encerrar ou um descanso longo`;
}

/** The label on the master's row: "Ajuda +5 PV". */
export function aidRowLabel(bonus: number): string {
  return `Ajuda +${bonus} PV`;
}

/** What a screen reader reads for the hit points: "Pontos de vida: 43 de 43, 5 de Ajuda". */
export function vitalsSpeech(current: number, max: number, aid: number): string {
  return `Pontos de vida: ${current} de ${max}${aid > 0 ? `, ${aid} de Ajuda` : ''}`;
}

/** The bar's two pieces, in percent of the effective maximum: the solid part (up to the sheet's maximum) and
 * the striped part (what is filled of Ajuda's). */
export function aidBar(
  current: number,
  max: number,
  aid: number,
): { readonly solid: number; readonly striped: number } {
  if (max <= 0) {
    return { solid: 0, striped: 0 };
  }
  const own = Math.max(0, max - Math.max(0, aid));
  const solid = Math.min(Math.max(0, current), own);
  const striped = Math.min(Math.max(0, current - own), Math.max(0, aid));
  return { solid: (solid / max) * 100, striped: (striped / max) * 100 };
}

/** The sheet's own maximum under Ajuda: "máximo 38 da ficha". */
export function sheetMaximum(max: number, aid: number): number {
  return max - Math.max(0, aid);
}

/** The question when the master ends Ajuda, in the order the board says it: the maximum goes back, and what
 * the current hit points do (only what passes the new maximum is lost, and that is not undone). */
export function endAidText(current: number, max: number, aid: number): string {
  const back = sheetMaximum(max, aid);
  const first = `O máximo de PV volta a ${back}.`;
  return current > back
    ? `${first} Os PV atuais passam de ${current} para ${back}: o que passa do novo máximo se perde, e isto não se desfaz.`
    : `${first} Os PV atuais ficam em ${current}: só o máximo cai, e isto não se desfaz.`;
}
