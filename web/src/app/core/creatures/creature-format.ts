import { CreatureSource } from '../../../gen/meurpg/characters/v1/characters_pb';
import type { Creature, CreatureSummary } from '../../../gen/meurpg/rules/v1/rules_pb';
import { joinDots, tight } from '../format/text';
import { metersText } from '../units';

/** How the table says where a creature came from, under its name: "Familiar de Pensantus". */
export function sourcePhrase(source: CreatureSource, ownerName: string): string {
  switch (source) {
    case CreatureSource.FAMILIAR:
      return `Familiar de ${ownerName}`;
    case CreatureSource.ANIMATE_DEAD:
      return `Animado por ${ownerName}`;
    case CreatureSource.CONJURE_ANIMALS:
      return `Conjurado por ${ownerName}`;
    case CreatureSource.MASTER:
      return 'Dado pelo mestre';
    default:
      return `Criatura de ${ownerName}`;
  }
}

/** The short origin of the master's list: "familiar", "dado pelo mestre". */
export function sourceShort(source: CreatureSource): string {
  switch (source) {
    case CreatureSource.FAMILIAR:
      return 'familiar';
    case CreatureSource.ANIMATE_DEAD:
      return 'animado';
    case CreatureSource.CONJURE_ANIMALS:
      return 'conjurado';
    default:
      return 'dado pelo mestre';
  }
}

/** The speeds as the card and the form list say them: "3 m, voo 15 m", "nada 9 m", "1,5 m, voo 18 m". */
export function speedsText(c: Pick<Creature, 'speedWalkFt' | 'speedFlyFt' | 'speedSwimFt' | 'speedClimbFt' | 'speedBurrowFt'>): string {
  const parts: string[] = [];
  if (c.speedWalkFt > 0) {
    parts.push(metersText(c.speedWalkFt));
  }
  const others: [number, string][] = [
    [c.speedClimbFt, 'escala'],
    [c.speedSwimFt, 'nada'],
    [c.speedFlyFt, 'voo'],
    [c.speedBurrowFt, 'escava'],
  ];
  for (const [ft, word] of others) {
    if (ft > 0) {
      parts.push(tight(`${word} ${metersText(ft)}`));
    }
  }
  return parts.join(', ') || '0 m';
}

/** The challenge rating as the list says it: "ND 1/8". */
export function challengeText(cr: string): string {
  return `ND ${cr}`;
}

/** A form's line in a choice list: "Miúdo · 1 PV · 6 m, escala 6 m". */
export function formSubtitle(c: Creature): string {
  return joinDots([c.summary?.sizePt ?? '', tight(`${c.hitPoints} PV`), speedsText(c)]);
}

/** A catalog row's line: "Médio · fera · ND 1/8", with the AC and hit points when the stat block is known. */
export function summarySubtitle(s: CreatureSummary, c?: Creature): string {
  const parts = [s.sizePt, s.typePt, challengeText(s.challengeRating)];
  if (c) {
    parts.push(`CA ${c.armorClass}`, tight(`PV ${c.hitPoints}`));
  }
  return joinDots(parts);
}

/** An ability modifier with the true minus sign, as the sheet writes it: "+2", "−4". */
export function signed(n: number): string {
  return n >= 0 ? `+${n}` : `−${-n}`;
}

/** "2 de 40", the counter of a name field. */
export function nameCounter(length: number, max: number): string {
  return `${length} de ${max}`;
}

/** The longest name a creature may have (`RenameCreature`, `GiveCreature`). */
export const CREATURE_NAME_MAX = 40;
