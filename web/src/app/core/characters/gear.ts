import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import { CharacterService, type ItemBonus } from '../../../gen/meurpg/characters/v1/characters_pb';
import { type GearEntry, GearKind, type ItemModifier } from '../../../gen/meurpg/rules/v1/rules_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** What a weapon is made of or enchanted with, for the resistances and immunities to nonmagical attacks. */
export interface MarksVm {
  readonly silvered: boolean;
  readonly adamantine: boolean;
  readonly magical: boolean;
}

/** The marks that are set, as the words a tag or a row shows, in the order of the grid. */
export function markWords(m: MarksVm | null | undefined): string[] {
  return [m?.silvered && 'Prateada', m?.adamantine && 'Adamantina', m?.magical && 'Mágica'].filter(
    (w): w is string => !!w,
  );
}

/** One piece of gear a sheet lists, equipped or carried (`DerivedSheet.gear`): the names and the armor class come from
 * the server, never from the browser. */
export interface GearVm {
  readonly key: string;
  readonly namePt: string;
  readonly kind: 'armor' | 'shield' | 'weapon';
  readonly equipped: boolean;
  readonly twoHanded: boolean;
  /** The armor class with this piece's state flipped (0: the server does not preview it, e.g. an armor spell). */
  readonly armorClassIfToggled: number;
  /** A weapon's marks; absent for armor and shield. */
  readonly marks?: MarksVm;
  /** How many copies the sheet lists: weapons with one key are one row, their state kept by key. */
  readonly count: number;
  /** The magic bonus stored for this piece's key (1 to 3); absent when the piece has none. */
  readonly bonus?: number;
  /** Whether the bonus is added to the numbers now: every modifier the piece gives counts. */
  readonly bonusCounts?: boolean;
  /** Why the bonus does not count now, in the server's words; absent when it counts or the piece has no bonus. */
  readonly bonusReason?: string;
}

/** The accessible name of the magic bonus chip; `withWords` gives the words the attack sheet shows. */
export function bonusChipName(bonus: number, counts: boolean, withWords = false): string {
  if (withWords) {
    return counts ? 'Bônus mágico, já somado' : 'Bônus mágico, não conta agora';
  }
  return `Bônus mágico +${bonus}, ${counts ? 'já somado' : 'não conta agora'}`;
}

/** The marks an attack row or sheet shows: a counting +N takes the place of "Mágica". */
export function attackMarksFor(
  marks: MarksVm | undefined,
  countingBonus: number,
): MarksVm | undefined {
  return marks && countingBonus > 0 ? { ...marks, magical: false } : marks;
}

const KINDS: Record<number, GearVm['kind']> = {
  [GearKind.ARMOR]: 'armor',
  [GearKind.SHIELD]: 'shield',
  [GearKind.WEAPON]: 'weapon',
};

/** A weapon's marks: an equipped weapon has them from its attack (the sheet's mark or a table rule), a carried one
 * from the sheet's mark; the same rule as the sheet page's. */
function withMarks(
  g: GearEntry,
  attacks: readonly MarkSource[],
  marks: readonly (MarkSource & { weaponKey: string })[],
  itemBonuses: readonly ItemBonus[] = [],
  itemModifiers: readonly ItemModifier[] = [],
): GearVm {
  const base = toGearVm(g);
  const stored = itemBonuses.find((b) => b.key === g.key)?.bonus;
  const own = itemModifiers.filter((m) => m.sourceKey === g.key);
  const reason = own.find((m) => !m.counts)?.reasonPt;
  const vm: GearVm = stored
    ? {
        ...base,
        bonus: stored,
        bonusCounts: own.every((m) => m.counts),
        ...(reason ? { bonusReason: reason } : {}),
      }
    : base;
  if (vm.kind !== 'weapon') {
    return vm;
  }
  const source = attacks.find((a) => a.key === g.key) ?? marks.find((m) => m.weaponKey === g.key);
  return source
    ? {
        ...vm,
        marks: {
          silvered: source.silvered,
          adamantine: source.adamantine,
          magical: source.magical,
        },
      }
    : vm;
}

/** The gear rows of a sheet with their weapons' marks (see `withMarks`). */
export function gearWithMarks(
  gear: readonly GearEntry[],
  attacks: readonly MarkSource[],
  marks: readonly (MarkSource & { weaponKey: string })[],
  itemBonuses: readonly ItemBonus[] = [],
  itemModifiers: readonly ItemModifier[] = [],
): GearVm[] {
  return gear.map((g) => withMarks(g, attacks, marks, itemBonuses, itemModifiers));
}

interface MarkSource {
  readonly key?: string;
  readonly silvered: boolean;
  readonly adamantine: boolean;
  readonly magical: boolean;
}

export function toGearVm(g: GearEntry): GearVm {
  return {
    key: g.key,
    namePt: g.namePt,
    kind: KINDS[g.kind] ?? 'weapon',
    equipped: g.equipped,
    twoHanded: g.twoHanded,
    armorClassIfToggled: g.armorClassIfToggled,
    count: Math.max(1, g.count),
  };
}

/** What a read of a character's gear holds: the list, the armor class now, whether the shield counts for nothing (a
 * two-handed weapon is equipped) and the sheet's revision. */
export interface GearRead {
  readonly gear: GearVm[];
  readonly armorClass: number;
  readonly shieldIdle: boolean;
  readonly revision: number;
}

/** The gear of a character and the change of it (`SetEquipped`), for the screens that are not the sheet page (the
 * combat). `providedIn: 'root'`, imported only by lazy code; tests replace it with `{ provide: GearClient, useValue }`. */
@Injectable({ providedIn: 'root' })
export class GearClient {
  private readonly client = createClient(CharacterService, inject(CONNECT_TRANSPORT));

  /** The gear and the armor class now, as the server derives them. */
  async read(campaignId: string, characterId: string): Promise<GearRead> {
    const res = await this.client.getCharacter({ campaignId, characterId });
    const derived = res.character?.derived;
    const content = res.character?.sheet?.content;
    const full = content?.case === 'full' ? content.value : null;
    return {
      gear: (derived?.gear ?? []).map((g) =>
        withMarks(
          g,
          derived?.attacks ?? [],
          full?.weaponMarks ?? [],
          full?.itemBonuses,
          derived?.itemModifiers,
        ),
      ),
      armorClass: derived?.armorClass ?? 0,
      shieldIdle: derived?.shieldIdle ?? false,
      revision: res.character?.revision ?? 0,
    };
  }

  async set(
    campaignId: string,
    characterId: string,
    key: string,
    equipped: boolean,
  ): Promise<void> {
    await this.client.setEquipped({ campaignId, characterId, key, equipped });
  }
}
