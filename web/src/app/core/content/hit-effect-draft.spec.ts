import { create } from '@bufbuild/protobuf';

import { Ability } from '../../../gen/meurpg/rules/v1/rules_pb';
import {
  TableEffectApplies,
  TableEffectAudience,
  TableEffectModifierKind,
  TableEffectModifierSchema,
  TableHitConditionDuration,
  TableHitTrigger,
  TableSpellEffectSchema,
  TableSpellSchema,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import {
  applyHit,
  emptyHit,
  hitAbilityKey,
  hitAvailable,
  hitFromEffect,
  hitIndex,
  hitPath,
  hitRefusals,
  hitToModifier,
  markedLock,
  needsConcentration,
  settleHit,
} from './hit-effect-draft';
import { draftToSpell, spellToDraft, withTarget } from './spell-draft';
import { emptySpell } from './spell-types';

const hitMod = (extra = {}) =>
  create(TableEffectModifierSchema, {
    kind: TableEffectModifierKind.WEAPON_HIT_DAMAGE,
    trigger: TableHitTrigger.NEXT_MELEE_HIT,
    die: 8,
    dice: 2,
    perSlotDice: 1,
    maxDice: 5,
    damageType: 'damage-type:cold',
    save: 'str',
    onFailConditions: ['condition:restrained'],
    conditionDuration: TableHitConditionDuration.UNTIL_SPELL_END,
    ...extra,
  });
const otherMod = (value: number) =>
  create(TableEffectModifierSchema, { kind: TableEffectModifierKind.ROLL_BONUS, value });

describe('the hit effect draft (#577)', () => {
  it('reads the weapon-hit modifier of a stored effect', () => {
    const effect = create(TableSpellEffectSchema, { modifiers: [otherMod(1), hitMod()] });
    expect(hitIndex(effect)).toBe(1);
    expect(hitFromEffect(effect)).toEqual({
      enabled: true,
      trigger: 'next_melee_hit',
      dice: '2',
      die: 8,
      perSlot: '1',
      cap: '5',
      damageType: 'damage-type:cold',
      saveAbility: 'str',
      condition: 'condition:restrained',
      extraConditions: [],
      conditionLasts: 'spell_end',
    });
  });

  it('is off for a spell with no effect, or an effect with no hit modifier', () => {
    expect(hitFromEffect(undefined).enabled).toBe(false);
    expect(
      hitFromEffect(create(TableSpellEffectSchema, { modifiers: [otherMod(1)] })).enabled,
    ).toBe(false);
  });

  it('writes the modifier with the fields the weapon hit reads', () => {
    const m = hitToModifier({
      ...emptyHit(),
      enabled: true,
      trigger: 'marked_target',
      dice: '3',
      die: 10,
      perSlot: '2',
      cap: '9',
    });
    expect(m).toMatchObject({
      kind: TableEffectModifierKind.WEAPON_HIT_DAMAGE,
      trigger: TableHitTrigger.MARKED_TARGET,
      die: 10,
      dice: 3,
      perSlotDice: 2,
      maxDice: 9,
      damageType: '',
      save: '',
      onFailConditions: [],
      conditionDuration: TableHitConditionDuration.UNSPECIFIED,
    });
  });

  it('sends the condition and its duration only with a save', () => {
    const base = { ...emptyHit(), enabled: true, cap: '4', condition: 'condition:blinded' };
    expect(hitToModifier(base)).toMatchObject({ save: '', onFailConditions: [] });
    const m = hitToModifier({ ...base, saveAbility: 'con', conditionLasts: 'next_turn' });
    expect(m).toMatchObject({
      save: 'con',
      onFailConditions: ['condition:blinded'],
      conditionDuration: TableHitConditionDuration.UNTIL_END_OF_NEXT_TURN,
    });
  });

  it('creates an effect with only this modifier when the spell has none', () => {
    const out = applyHit(undefined, { ...emptyHit(), enabled: true, cap: '3' }, true);
    expect(out?.modifiers).toHaveLength(1);
    expect(out?.modifiers[0].kind).toBe(TableEffectModifierKind.WEAPON_HIT_DAMAGE);
    expect(out?.conditions).toEqual([]);
  });

  it('writes in place and keeps every other part of the effect', () => {
    const effect = create(TableSpellEffectSchema, {
      applies: TableEffectApplies.ALL,
      conditions: ['condition:blinded'],
      playerLabelPt: 'Sinal',
      visibility: 1,
      modifiers: [otherMod(1), hitMod(), otherMod(2)],
    });
    const hit = { ...hitFromEffect(effect), dice: '4' };
    const out = applyHit(effect, hit, true);
    expect(out?.modifiers.map((m) => m.kind)).toEqual([
      TableEffectModifierKind.ROLL_BONUS,
      TableEffectModifierKind.WEAPON_HIT_DAMAGE,
      TableEffectModifierKind.ROLL_BONUS,
    ]);
    expect(out?.modifiers[1].dice).toBe(4);
    expect(out?.modifiers[0]).toEqual(effect.modifiers[0]);
    expect(out).toMatchObject({
      applies: TableEffectApplies.ALL,
      conditions: ['condition:blinded'],
      playerLabelPt: 'Sinal',
      visibility: 1,
    });
    // the stored effect is not touched
    expect(effect.modifiers[1].dice).toBe(2);
  });

  it('appends the modifier after the others when it is new', () => {
    const effect = create(TableSpellEffectSchema, { modifiers: [otherMod(1)] });
    const out = applyHit(effect, { ...emptyHit(), enabled: true, cap: '2' }, true);
    expect(out?.modifiers.map((m) => m.kind)).toEqual([
      TableEffectModifierKind.ROLL_BONUS,
      TableEffectModifierKind.WEAPON_HIT_DAMAGE,
    ]);
  });

  it('takes out only its own modifier when turned off', () => {
    const effect = create(TableSpellEffectSchema, {
      conditions: ['condition:blinded'],
      modifiers: [otherMod(1), hitMod(), otherMod(2)],
    });
    const out = applyHit(effect, { ...hitFromEffect(effect), enabled: false }, true);
    expect(out?.modifiers.map((m) => m.value)).toEqual([1, 2]);
    expect(out?.conditions).toEqual(['condition:blinded']);
  });

  it('drops an effect it leaves empty, and leaves alone one it never had', () => {
    const only = create(TableSpellEffectSchema, { modifiers: [hitMod()] });
    expect(applyHit(only, { ...hitFromEffect(only), enabled: false }, true)).toBeUndefined();
    expect(applyHit(undefined, emptyHit(), true)).toBeUndefined();
    const other = create(TableSpellEffectSchema, { modifiers: [otherMod(1)] });
    expect(applyHit(other, emptyHit(), true)).toBe(other);
  });

  it('drops an effect with no conditions and no modifiers left, whatever else it carries', () => {
    const effect = create(TableSpellEffectSchema, {
      modifiers: [hitMod()],
      visibility: TableEffectAudience.PUBLIC,
      playerLabelPt: 'Sinal',
    });
    expect(applyHit(effect, { ...hitFromEffect(effect), enabled: false }, true)).toBeUndefined();
    const kept = create(TableSpellEffectSchema, {
      conditions: ['condition:blinded'],
      modifiers: [hitMod()],
      playerLabelPt: 'Sinal',
    });
    expect(applyHit(kept, { ...hitFromEffect(kept), enabled: false }, true)?.playerLabelPt).toBe(
      'Sinal',
    );
  });

  it('knows which triggers last as long as the concentration', () => {
    expect(needsConcentration('each_hit')).toBe(true);
    expect(needsConcentration('marked_target')).toBe(true);
    expect(needsConcentration('next_hit')).toBe(false);
    expect(needsConcentration('next_melee_hit')).toBe(false);
    expect(needsConcentration('')).toBe(false);
  });

  it('keeps a stored trigger it does not know as unset, and refuses to save it', () => {
    const effect = create(TableSpellEffectSchema, {
      modifiers: [hitMod({ trigger: TableHitTrigger.UNSPECIFIED })],
    });
    const h = hitFromEffect(effect);
    expect(h.trigger).toBe('');
    expect(hitToModifier(h).trigger).toBe(TableHitTrigger.UNSPECIFIED);
    const refusals = hitRefusals(effect, h, true);
    expect(refusals).toEqual([
      { path: 'table_spell.effect.modifiers[0].trigger', text: 'Escolha quando vale.' },
    ]);
    expect(hitRefusals(effect, { ...h, trigger: 'next_hit' }, true)).toEqual([]);
    expect(hitRefusals(effect, h, false)).toEqual([]);
    expect(settleHit({ ...emptySpell(), hit: h }).hit.trigger).toBe('');
  });

  it('sends no hit modifier while the spell does not last, and keeps the form on', () => {
    const effect = create(TableSpellEffectSchema, { modifiers: [otherMod(1), hitMod()] });
    const out = applyHit(effect, hitFromEffect(effect), false);
    expect(out?.modifiers).toHaveLength(1);
    expect(hitAvailable({ duration: 'instant' })).toBe(false);
    expect(hitAvailable({ duration: 'timed' })).toBe(true);
    expect(hitAvailable({ duration: 'until_dispelled' })).toBe(true);
  });

  it('locks the marked target unless the target is one creature with no extra targets', () => {
    expect(markedLock({ target: 'creature', perSlot: 0 })).toBe('');
    expect(markedLock({ target: 'area', perSlot: 0 })).toBe('Pede o alvo Uma criatura.');
    expect(markedLock({ target: 'creatures', perSlot: 0 })).toBe('Pede o alvo Uma criatura.');
    expect(markedLock({ target: 'self', perSlot: 0 })).toBe('Pede o alvo Uma criatura.');
    expect(markedLock({ target: 'creature', perSlot: 1 })).toContain('sem criaturas a mais');
  });

  it('moves a locked marked target to the next hit, and leaves other triggers', () => {
    const d = { ...emptySpell(), hit: { ...emptyHit(), trigger: 'marked_target' as const } };
    expect(settleHit(d).hit.trigger).toBe('marked_target');
    expect(settleHit({ ...d, target: 'area' as const }).hit.trigger).toBe('next_hit');
    const each = { ...d, target: 'area' as const, hit: { ...d.hit, trigger: 'each_hit' as const } };
    expect(settleHit(each).hit.trigger).toBe('each_hit');
    expect(withTarget(d, 'creatures').hit.trigger).toBe('next_hit');
    expect(withTarget(d, 'creature').hit.trigger).toBe('marked_target');
  });

  it('puts the refusals at the modifier where it will be sent', () => {
    const effect = create(TableSpellEffectSchema, { modifiers: [otherMod(1), otherMod(2)] });
    expect(hitPath(effect, 'dice')).toBe('table_spell.effect.modifiers[2].dice');
    expect(hitPath(undefined, 'dice')).toBe('table_spell.effect.modifiers[0].dice');
    const stored = create(TableSpellEffectSchema, { modifiers: [otherMod(1), hitMod()] });
    expect(hitPath(stored, 'max_dice')).toBe('table_spell.effect.modifiers[1].max_dice');
  });

  it('asks for the cap, which the server requires, and for the condition of a save', () => {
    const on = { ...emptyHit(), enabled: true };
    expect(hitRefusals(undefined, on, true).map((r) => r.path)).toEqual([
      'table_spell.effect.modifiers[0].max_dice',
    ]);
    expect(hitRefusals(undefined, { ...on, cap: '4' }, true)).toEqual([]);
    const save = hitRefusals(undefined, { ...on, cap: '4', saveAbility: 'con' }, true);
    expect(save.map((r) => r.path)).toEqual(['table_spell.effect.modifiers[0].on_fail_conditions']);
    expect(save[0].text).toBe('Escolha a condição de quem falhar, ou tire o teste.');
    expect(hitRefusals(undefined, { ...on, cap: 'x', dice: '1,5' }, true)).toHaveLength(2);
    expect(hitRefusals(undefined, on, false)).toEqual([]);
    expect(hitRefusals(undefined, emptyHit(), true)).toEqual([]);
  });

  it('names the ability keys of the wire', () => {
    expect(hitAbilityKey(Ability.CONSTITUTION)).toBe('con');
    expect(hitAbilityKey(Ability.STRENGTH)).toBe('str');
  });
});

describe('a saved spell with a hit effect', () => {
  const stored = create(TableSpellSchema, {
    namePt: 'Clarão',
    level: 1,
    schoolKey: 'school:evocation',
    duration: { kind: 2, amount: 1, unit: 2 },
    concentration: true,
    target: { kind: 1 },
    effect: {
      conditions: ['condition:blinded'],
      modifiers: [otherMod(1), hitMod({ trigger: TableHitTrigger.EACH_HIT })],
    },
  });

  it('loads into the form and goes back unchanged', () => {
    const d = spellToDraft(stored);
    expect(d.hit.enabled).toBe(true);
    expect(d.hit.trigger).toBe('each_hit');
    const sent = draftToSpell(d);
    expect(sent.effect).toEqual(stored.effect);
  });

  it('sends the edited modifier among the others', () => {
    const d = spellToDraft(stored);
    const sent = draftToSpell({ ...d, hit: { ...d.hit, dice: '3', cap: '8' } });
    const effect = create(TableSpellEffectSchema, sent.effect as never);
    expect(effect.modifiers).toHaveLength(2);
    expect(effect.modifiers[1]).toMatchObject({ dice: 3, maxDice: 8 });
    expect(effect.conditions).toEqual(['condition:blinded']);
  });

  it('sends no effect for a spell that never had one and leaves the block off', () => {
    expect(draftToSpell(emptySpell()).effect).toBeUndefined();
  });

  it('drops the modifier from an instantaneous spell', () => {
    const d = spellToDraft(stored);
    const sent = draftToSpell({ ...d, duration: 'instant' });
    expect(create(TableSpellEffectSchema, sent.effect as never).modifiers).toHaveLength(1);
  });
});
