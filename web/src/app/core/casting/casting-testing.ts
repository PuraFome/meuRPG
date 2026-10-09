import { create, type MessageInitShape } from '@bufbuild/protobuf';

import {
  type CastSpellOutsideCombatResponse,
  type GetCastOptionsResponse,
  type ListSpellCastsResponse,
  type OutsideCast,
  CastingSpellSchema,
  CastingTargetSchema,
  CastingEffectKind,
  GetCastOptionsResponseSchema,
  ListSpellCastsResponseSchema,
  OutsideCastSchema,
  OutsideCastStatus,
} from '../../../gen/meurpg/play/v1/casting_pb';
import { SlotChoiceSchema, SpellSchema } from '../../../gen/meurpg/rules/v1/rules_pb';
import type { CastDice, OutsideCastRequest } from './casting-client';

/** A spell of "Conjurar" for the specs. */
export function castingSpell(
  key: string,
  namePt: string,
  level: number,
  extra: Record<string, unknown> = {},
) {
  return create(CastingSpellSchema, {
    spell: create(SpellSchema, { key, name: namePt, namePt, level }),
    canCast: true,
    slots: level > 0 ? [create(SlotChoiceSchema, { level, free: 2 })] : [],
    castingTimePt: '1 ação',
    effect: CastingEffectKind.NARRATED,
    ...extra,
  });
}

export function castingTarget(characterId: string, name: string, npc = false) {
  return create(CastingTargetSchema, { characterId, name, npc });
}

export function outsideCast(extra: MessageInitShape<typeof OutsideCastSchema> = {}): OutsideCast {
  return create(OutsideCastSchema, {
    id: 'cast-1',
    spellKey: 'spell:bless',
    spellNamePt: 'Bênção',
    casterId: 'ilaria',
    casterName: 'Ilaria',
    status: OutsideCastStatus.ACTIVE,
    ...extra,
  });
}

/** A `CastingClient` that answers from what the spec sets and records what it was asked. */
export class FakeCastingClient {
  options_: GetCastOptionsResponse = create(GetCastOptionsResponseSchema, {});
  active: OutsideCast[] = [];
  log: OutsideCast[] = [];
  cast_: OutsideCast = outsideCast();
  failWith: unknown = null;
  readonly casts: { req: OutsideCastRequest; key: string }[] = [];
  readonly finished: { castId: string; dice: CastDice | null; key: string }[] = [];
  readonly stopped: string[] = [];
  readonly ended: string[] = [];

  options(): Promise<GetCastOptionsResponse> {
    return Promise.resolve(this.options_);
  }

  list(): Promise<ListSpellCastsResponse> {
    return Promise.resolve(
      create(ListSpellCastsResponseSchema, { active: this.active, log: this.log }),
    );
  }

  cast(req: OutsideCastRequest, key: string): Promise<CastSpellOutsideCombatResponse> {
    if (this.failWith) {
      return Promise.reject(this.failWith);
    }
    this.casts.push({ req, key });
    return Promise.resolve({ cast: this.cast_ } as CastSpellOutsideCombatResponse);
  }

  confirm(
    _campaignId: string,
    castId: string,
    dice: CastDice | null,
    key: string,
  ): Promise<unknown> {
    if (this.failWith) {
      return Promise.reject(this.failWith);
    }
    this.finished.push({ castId, dice, key });
    return Promise.resolve({});
  }

  abandon(_campaignId: string, castId: string): Promise<unknown> {
    this.stopped.push(castId);
    return Promise.resolve({});
  }

  end(_campaignId: string, castId: string): Promise<unknown> {
    this.ended.push(castId);
    return Promise.resolve({});
  }
}
