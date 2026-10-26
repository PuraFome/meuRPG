import {
  type CharacterVitals,
  type ImageAudience,
  ImageAudienceKind,
  type ShownImage,
} from '../../../gen/meurpg/play/v1/play_pb';
import { Recharge } from '../../../gen/meurpg/rules/v1/rules_pb';
import { hitDiceWords } from '../resources/hit-dice-text';
import type { ImageAudienceVm, ShownImageVm, VitalsVm } from './live-session.types';

const RECHARGE: Partial<Record<Recharge, 'short_rest' | 'long_rest' | 'dawn' | 'none'>> = {
  [Recharge.SHORT_REST]: 'short_rest',
  [Recharge.LONG_REST]: 'long_rest',
  [Recharge.DAWN]: 'dawn',
};

export function toVitalsVm(v: CharacterVitals): VitalsVm {
  return {
    characterId: v.characterId,
    name: v.name,
    playerUserId: v.playerUserId,
    hitPointsCurrent: v.hitPointsCurrent,
    hitPointsMax: v.hitPointsMax,
    hitPointsTemporary: v.hitPointsTemporary,
    hitPointsMaxBonus: v.hitPointsMaxBonus,
    exhaustionLevel: v.exhaustionLevel,
    armorClassBase: v.armorClassBase,
    arcaneRecoveryAllowance: v.arcaneRecoveryAllowance,
    spellSlots: v.spellSlots.map((s) => ({
      level: s.level,
      total: s.total,
      used: s.used,
      created: s.created,
    })),
    pactSlots: v.pactSlots
      ? { slotLevel: v.pactSlots.slotLevel, total: v.pactSlots.total, used: v.pactSlots.used }
      : null,
    hitDice: hitDiceWords(v.hitDice),
    hitDiceSizes: v.hitDice.map((hd) => ({
      faces: hd.faces,
      total: hd.count,
      used: v.hitDiceUsedByDie[hd.faces] ?? 0,
    })),
    hitDiceTotal: v.hitDiceTotal,
    hitDiceUsed: v.hitDiceUsed,
    revision: v.revision,
    familiarSight: v.familiarSight
      ? { creatureId: v.familiarSight.creatureId, inCombat: v.familiarSight.inCombat }
      : null,
    resources: v.resources.map((r) => ({
      key: r.key,
      namePt: r.namePt,
      total: r.total,
      used: r.used,
      recharge: RECHARGE[r.recharge] ?? 'none',
    })),
    wildShape: v.wildShape
      ? {
          beastKey: v.wildShape.beastKey,
          beastNamePt: v.wildShape.beastNamePt,
          hitPointsCurrent: v.wildShape.hitPointsCurrent,
          hitPointsMax: v.wildShape.hitPointsMax,
        }
      : null,
  };
}

/** The audience a shown image carries for the master (`undefined` for a player's copy, which has none). */
export function toImageAudienceVm(
  audience: ImageAudience | undefined,
): ImageAudienceVm | undefined {
  if (!audience) {
    return undefined;
  }
  const kind =
    audience.kind === ImageAudienceKind.SCENE_PLAYERS
      ? 'scene'
      : audience.kind === ImageAudienceKind.CHOSEN_PLAYERS
        ? 'chosen'
        : 'whole';
  return { kind, userIds: [...audience.userIds] };
}

export function toShownImageVm(image: ShownImage | undefined): ShownImageVm | null {
  if (!image || !image.id) {
    return null;
  }
  const audience = toImageAudienceVm(image.audience);
  return {
    id: image.id,
    name: image.name,
    width: image.width,
    height: image.height,
    url: image.url,
    ...(audience ? { audience } : {}),
  };
}

/** The audience the master asks for, as the request carries it. */
export function imageAudienceOf(audience: ImageAudienceVm): {
  kind: ImageAudienceKind;
  userIds: string[];
} {
  const kind =
    audience.kind === 'scene'
      ? ImageAudienceKind.SCENE_PLAYERS
      : audience.kind === 'chosen'
        ? ImageAudienceKind.CHOSEN_PLAYERS
        : ImageAudienceKind.WHOLE_TABLE;
  return { kind, userIds: audience.kind === 'chosen' ? [...audience.userIds] : [] };
}
