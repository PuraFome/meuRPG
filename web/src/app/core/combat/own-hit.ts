import { article } from '../format/article';

/** The latest attack that hurt the player's own character, as the server told it to them (`attack_hit`). */
export interface OwnHit {
  /** The pending damage that was applied, so a hit is announced once. */
  readonly id: string;
  /** "A Espada voadora 2 acertou você: 9 de dano cortante." */
  readonly text: string;
}

/** What `attack_hit` carries (play.proto). */
export interface AttackHit {
  readonly id: string;
  /** The attacker's name; empty when it is hidden from this player. */
  readonly actorLabel: string;
  readonly amount: number;
  /** "cortante", "fogo"; empty when the hit has no type. */
  readonly damageTypePt: string;
}

/** "dano cortante", "dano de fogo". */
function damageWords(typePt: string): string {
  const name = typePt.toLowerCase();
  if (!name) {
    return 'dano';
  }
  return /^(cortante|perfurante|contundente)$/.test(name) ? `dano ${name}` : `dano de ${name}`;
}

/** The sentence the sheet says for the damage of an attack that landed: "A Espada voadora 2 acertou você: 9 de dano cortante.".
 * A hidden attacker is "Alguém": the player is told they were hit, not by what. */
export function ownHitOf(hit: AttackHit): OwnHit {
  const who = hit.actorLabel
    ? `${article(hit.actorLabel) === 'a' ? 'A' : 'O'} ${hit.actorLabel}`
    : 'Alguém';
  return {
    id: hit.id,
    text: `${who} acertou você: ${hit.amount} de ${damageWords(hit.damageTypePt)}.`,
  };
}
