import { vi } from 'vitest';
import { create } from '@bufbuild/protobuf';

import {
  type CharacterCreature,
  CharacterCreatureSchema,
  CreatureSource,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { SpellSlotUsageSchema, CharacterVitalsSchema } from '../../../gen/meurpg/play/v1/play_pb';
import {
  Ability,
  type Creature,
  CreatureAbilityScoreSchema,
  CreatureActionSchema,
  type CreatureSummary,
  CreatureSchema,
  CreatureSummarySchema,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import type { CreatureFilter, SummonCast } from './creatures-client';

type Init<T> = Partial<Omit<T, '$typeName' | '$unknown'>>;

/** A creature of a character, as `ListCharacterCreatures` sends it. */
export function creature(id: string, name: string, over: Init<CharacterCreature> = {}): CharacterCreature {
  return create(CharacterCreatureSchema, {
    id,
    characterId: 'char-1',
    monsterKey: 'monster:raven',
    monsterNamePt: 'Corvo',
    name,
    source: CreatureSource.FAMILIAR,
    attack: 1,
    hitPointsCurrent: 1,
    hitPointsMax: 1,
    ...over,
  });
}

/** A catalog row. */
export function summary(key: string, namePt: string, over: Init<CreatureSummary> = {}): CreatureSummary {
  return create(CreatureSummarySchema, {
    key,
    name: key.replace('monster:', ''),
    namePt,
    size: 'Tiny',
    sizePt: 'Miúdo',
    type: 'beast',
    typePt: 'fera',
    challengeRating: '0',
    ...over,
  });
}

/** The raven's stat block, with the numbers of the artboard (E9-10). */
export function raven(over: Init<Creature> = {}): Creature {
  const abilities = [2, 14, 8, 2, 12, 6].map((score, i) =>
    create(CreatureAbilityScoreSchema, {
      ability: i + 1 as Ability,
      namePt: ['Força', 'Destreza', 'Constituição', 'Inteligência', 'Sabedoria', 'Carisma'][i],
      score,
      modifier: Math.floor((score - 10) / 2),
    }),
  );
  return create(CreatureSchema, {
    summary: summary('monster:raven', 'Corvo'),
    armorClass: 12,
    hitPoints: 1,
    hitPointsRoll: '1d4-1',
    speedWalkFt: 10,
    speedFlyFt: 50,
    abilities,
    skills: [{ key: 'skill:perception', namePt: 'Percepção', bonus: 3 }],
    passivePerception: 13,
    traits: [{ name: 'Mimicry', text: 'The raven can mimic simple sounds it has heard.', usage: '' }],
    actions: [create(CreatureActionSchema, { name: 'Beak', text: 'Melee Weapon Attack: +4 to hit, reach 5 ft., one target. Hit: 1 piercing damage.', hasAttack: true, attackBonus: 4 })],
    ...over,
  });
}

/** Vitals with the slots the cast sheet reads. */
export function vitalsWith(characterId: string, slots: { level: number; total: number; used: number }[]) {
  return create(CharacterVitalsSchema, {
    characterId,
    spellSlots: slots.map((s) => create(SpellSlotUsageSchema, s)),
  });
}

/** The client of the creatures, with what each test needs and a record of what was called. */
export class FakeCreaturesClient {
  creatures: CharacterCreature[] = [];
  blocks = new Map<string, Creature>();
  catalog: CreatureSummary[] = [];
  vitals = vitalsWith('char-1', []);
  calls: string[] = [];
  casts: SummonCast[] = [];
  searches: CreatureFilter[] = [];
  failWith: unknown = null;

  list = vi.fn(async (_campaignId: string, _characterId: string) => {
    this.calls.push('list');
    if (this.failWith) {
      throw this.failWith;
    }
    return this.creatures;
  });
  give = vi.fn(async (_c: string, characterId: string, monsterKey: string, name: string) => {
    this.calls.push(`give ${monsterKey} ${name}`);
    if (this.failWith) {
      throw this.failWith;
    }
    const made = creature('new', name, { monsterKey, source: CreatureSource.MASTER, characterId });
    this.creatures = [...this.creatures, made];
    return made;
  });
  rename = vi.fn(async (_c: string, id: string, name: string) => {
    this.calls.push(`rename ${id} ${name}`);
    if (this.failWith) {
      throw this.failWith;
    }
    this.creatures = this.creatures.map((c) => (c.id === id ? { ...c, name } : c));
    return this.creatures.find((c) => c.id === id);
  });
  dismiss = vi.fn(async (_c: string, id: string) => {
    this.calls.push(`dismiss ${id}`);
    if (this.failWith) {
      throw this.failWith;
    }
    this.creatures = this.creatures.filter((c) => c.id !== id);
  });
  setHitPoints = vi.fn(async (_c: string, id: string, hp: number) => {
    this.calls.push(`hp ${id} ${hp}`);
    return this.creatures.find((c) => c.id === id);
  });
  search = vi.fn(async (_c: string, filter: CreatureFilter) => {
    this.searches.push(filter);
    const q = (filter.query ?? '').toLowerCase();
    const found = this.catalog.filter((s) => (q === '' || s.namePt.toLowerCase().includes(q) || s.name.toLowerCase().includes(q)) && (!filter.type || s.type === filter.type));
    return { creatures: found, total: found.length };
  });
  statBlock = vi.fn(async (_c: string, key: string) => {
    const b = this.blocks.get(key);
    if (!b) {
      throw new Error('no stat block');
    }
    return b;
  });
  vitalsOf = vi.fn(async () => this.vitals);
  castSummon = vi.fn(async (cast: SummonCast) => {
    this.casts.push(cast);
    if (this.failWith) {
      throw this.failWith;
    }
    return { creatureIds: ['new'], replacedIds: [] as string[] };
  });
}

/**
 * What a person reads in an element: the icons' ligature names are left out, and the
 * boxes the stylesheet puts side by side or one under the other are told apart with a
 * space ("CA 12", "Força −4 valor 2"), then the spaces are collapsed.
 */
export function flat(e: Element | null | undefined): string | undefined {
  if (!e) {
    return undefined;
  }
  const copy = e.cloneNode(true) as Element;
  copy.querySelectorAll('mat-icon').forEach((i) => i.remove());
  copy.querySelectorAll('*').forEach((n) => {
    n.insertAdjacentText('beforebegin', ' ');
    n.insertAdjacentText('afterend', ' ');
  });
  return (copy.textContent ?? '').replace(/\s+/g, ' ').replace(/ ([.,:;)])/g, '$1').replace(/\( /g, '(').trim();
}

/** Whether a button is off: a button that stays focusable says it with `aria-disabled`. */
export function isOff(b: HTMLButtonElement): boolean {
  return b.disabled || b.getAttribute('aria-disabled') === 'true';
}
