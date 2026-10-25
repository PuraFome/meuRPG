import type { CharacterVitals, ShownImage } from '../../../gen/meurpg/play/v1/play_pb';
import { Recharge } from '../../../gen/meurpg/rules/v1/rules_pb';
import { hitDiceWords } from '../resources/hit-dice-text';
import type { ShownImageVm, VitalsVm } from './live-session.types';

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
    states: v.activeStates.map((s) => ({
      key: s.key,
      namePt: s.namePt,
      textPt: s.textPt,
      durationRounds: s.durationRounds,
      roundsLeft: s.roundsLeft,
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

export function toShownImageVm(image: ShownImage | undefined): ShownImageVm | null {
  return image && image.id
    ? { id: image.id, name: image.name, width: image.width, height: image.height, url: image.url }
    : null;
}
