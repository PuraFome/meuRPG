import { create } from '@bufbuild/protobuf';

import {
  EncounterBlockedReason,
  EncounterBlockedSchema,
  ResourceTargetSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import {
  ActionEconomy,
  type ActionOption,
  ActionOptionSchema,
  FeatureTargetWho,
  UseScope,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import {
  asksChoice,
  asksSomething,
  asksTargets,
  choiceComplete,
  featureRefusalText,
  leadOf,
  missingText,
  pickable,
  scopeWord,
  targetLimitText,
  useButtonLabel,
  useLine,
  usedNote,
  useRows,
} from './feature-use';

/** The text with the no-break spaces of `tight()` written as plain ones, to read a sentence as it looks. */
const plain = (s: string) => s.replace(/ /g, ' ');

function option(over: Record<string, unknown> = {}): ActionOption {
  return create(ActionOptionSchema, {
    enabled: true,
    viaUseFeature: true,
    ...over,
    action: { key: 'feature:x', namePt: 'Característica', economy: ActionEconomy.ACTION, ...(over['action'] as object) },
  } as never);
}

const wild = { key: 'wild_shape', namePt: 'Forma Selvagem', total: 2, used: 0, recharge: 'short_rest' } as const;

describe('what a feature taken with UseFeature asks', () => {
  it('asks for creatures only when it picks some', () => {
    expect(asksTargets(option())).toBe(false);
    expect(asksTargets(option({ targetMax: 1, targetWho: FeatureTargetWho.SELF }))).toBe(false);
    expect(asksTargets(option({ targetMax: 3, targetMin: 1, targetWho: FeatureTargetWho.ALLY }))).toBe(true);
  });

  it('asks for an option when the feature offers some', () => {
    const o = option({ choices: [{ key: 'fire', textPt: 'Fogo' }] });
    expect(asksChoice(o)).toBe(true);
    expect(asksSomething(o)).toBe(true);
    expect(asksSomething(option())).toBe(false);
  });
});

describe('the line under a feature taken with UseFeature', () => {
  it('names the scope, what is left, the rest resource it spends and what it is left of', () => {
    const o = option({
      useScope: UseScope.COMBAT,
      useTotal: 1,
      useLeft: 1,
      usesLeft: 2,
      resourceTotal: 2,
      action: { resourceKey: 'wild_shape' },
    });
    expect(plain(useLine(o, wild))).toBe(
      '1 por combate · restam 1 de 1 · gasta 1 uso de Forma Selvagem (restam 2 de 2)',
    );
  });

  it('starts with a capital when the limit is not the first part, and says the temporary hit points', () => {
    const o = option({ usesLeft: 2, resourceTotal: 2, action: { resourceKey: 'wild_shape', tempHp: 16 } });
    expect(plain(useLine(o, wild))).toBe('Gasta 1 uso de Forma Selvagem (restam 2 de 2) · 16 PV temporários');
  });

  it('says whom it takes and how far, with the limit of the scope', () => {
    const o = option({
      useScope: UseScope.COMBAT,
      useTotal: 2,
      useLeft: 1,
      targetMin: 1,
      targetMax: 1,
      targetWho: FeatureTargetWho.ENEMY,
      targetRangeFt: 10,
    });
    expect(plain(useLine(o))).toBe('2 por combate · restam 1 de 2 · 1 inimigo a até 3 m');
  });

  it('says the scope word of each scope and none for a feature with no limit', () => {
    expect(scopeWord(UseScope.TURN)).toBe('por turno');
    expect(scopeWord(UseScope.ROUND)).toBe('por rodada');
    expect(scopeWord(UseScope.COMBAT)).toBe('por combate');
    expect(scopeWord(UseScope.UNSPECIFIED)).toBe('');
    expect(useLine(option())).toBe('');
  });

  it('writes the head of the list of creatures with the count and the range', () => {
    const o = option({ targetMin: 1, targetMax: 3, targetWho: FeatureTargetWho.ALLY, targetRangeFt: 30 });
    expect(plain(targetLimitText(o, 'head'))).toBe('de 1 a 3 aliados até 9 m');
    expect(plain(targetLimitText(o))).toBe('de 1 a 3 aliados a até 9 m');
    expect(plain(targetLimitText(option({ targetMin: 2, targetMax: 2, targetWho: FeatureTargetWho.ANY })))).toBe(
      '2 criaturas',
    );
    expect(targetLimitText(option())).toBe('');
  });
});

describe('the creatures of the list and the choice sent', () => {
  const target = (id: string, label: string, reason = '', tooFar = false) =>
    create(ResourceTargetSchema, {
      target: { combatantId: id, label, tooFar, distanceFt: 5 },
      disabledReasonPt: reason,
    });
  const o = option({ targetMin: 1, targetMax: 2, targetWho: FeatureTargetWho.ENEMY, targetRangeFt: 10 });

  it("keeps the server's reason on a creature that cannot be picked and picks the others", () => {
    const rows = useRows([target('a', 'Zumbi 1'), target('b', 'Zumbi 2', 'Fora de alcance: até 3 m')], o);
    expect(rows.map((r) => r.blocked)).toEqual(['', 'Fora de alcance: até 3 m']);
    expect(pickable(rows)).toEqual(['a']);
  });

  it('needs the number of creatures the limit says', () => {
    expect(choiceComplete(o, { targetIds: [], choice: '' })).toBe(false);
    expect(choiceComplete(o, { targetIds: ['a'], choice: '' })).toBe(true);
    expect(choiceComplete(o, { targetIds: ['a', 'b', 'c'], choice: '' })).toBe(false);
    expect(missingText(o, { targetIds: [], choice: '' })).toBe('Escolha de 1 a 2 inimigos até 3 m.');
  });

  it('needs one of the options when the feature offers them', () => {
    const c = option({ choices: [{ key: 'fire', textPt: 'Fogo' }, { key: 'cold', textPt: 'Frio' }] });
    expect(choiceComplete(c, { targetIds: [], choice: '' })).toBe(false);
    expect(choiceComplete(c, { targetIds: [], choice: 'acid' })).toBe(false);
    expect(choiceComplete(c, { targetIds: [], choice: 'fire' })).toBe(true);
    expect(missingText(c, { targetIds: [], choice: '' })).toBe('Escolha uma opção.');
    expect(missingText(c, { targetIds: [], choice: 'fire' })).toBe('');
  });

  it('writes the button with what will be sent', () => {
    const c = option({ choices: [{ key: 'fire', textPt: 'Fogo' }] });
    const label = (id: string) => ({ a: 'Zumbi 1', b: 'Brisa' })[id] ?? '';
    expect(useButtonLabel(c, { targetIds: [], choice: 'fire' }, label)).toBe('Usar: Fogo');
    expect(useButtonLabel(o, { targetIds: ['a'], choice: '' }, label)).toBe('Usar no Zumbi 1');
    expect(useButtonLabel(o, { targetIds: ['b'], choice: '' }, label)).toBe('Usar na Brisa');
    expect(useButtonLabel(o, { targetIds: ['a', 'b'], choice: '' }, label)).toBe('Usar em 2');
    expect(useButtonLabel(option(), { targetIds: [], choice: '' }, label)).toBe('Usar');
  });
});

describe('the note after a use', () => {
  it('says what was given and what is left of the resource', () => {
    const o = option({ usesLeft: 2, resourceTotal: 2, action: { resourceKey: 'wild_shape' } });
    expect(plain(usedNote('Entidade simbiótica', { healed: 0, tempHp: 16 }, o, wild))).toBe(
      'Entidade simbiótica usada: 16 PV temporários. Resta 1 de 2 usos de Forma Selvagem.',
    );
  });

  it('says only that it was used when it gave nothing and spent no resource', () => {
    expect(usedNote('Corrigir o tipo de dano', { healed: 0, tempHp: 0 }, option(), undefined)).toBe(
      'Corrigir o tipo de dano usada.',
    );
  });

  it('says the heal too', () => {
    expect(usedNote('Cura', { healed: 7, tempHp: 3 }, option(), undefined)).toBe(
      'Cura usada: 7 PV recuperados e 3 PV temporários.',
    );
  });
});

describe('the refusals of a target and of a choice', () => {
  const blocked = (reason: EncounterBlockedReason, combatantIds: string[] = []) =>
    create(EncounterBlockedSchema, { reason, combatantIds });
  const names = new Map([
    ['a', 'Zumbi 1'],
    ['b', 'Brisa'],
  ]);
  const label = (id: string) => names.get(id);

  it('names the creature that was refused, and that nothing was spent is the rule of the server', () => {
    expect(featureRefusalText(blocked(EncounterBlockedReason.FEATURE_TARGET, ['a']), label)).toBe(
      'O Zumbi 1 saiu do alcance. Escolha outro alvo.',
    );
    expect(featureRefusalText(blocked(EncounterBlockedReason.FEATURE_TARGET, ['a', 'b']), label)).toBe(
      'O Zumbi 1 e a Brisa saíram do alcance. Escolha outro alvo.',
    );
  });

  it('says it without a name when the count or the side is wrong', () => {
    expect(featureRefusalText(blocked(EncounterBlockedReason.FEATURE_TARGET), label)).toBe(
      'O alvo saiu do alcance. Escolha outro alvo.',
    );
  });

  it('says the choice is no longer offered', () => {
    expect(featureRefusalText(blocked(EncounterBlockedReason.FEATURE_CHOICE, ['a']), label)).toBe(
      'Essa opção não vale mais. Escolha outra.',
    );
  });

  it('leaves the other reasons to the general text', () => {
    expect(featureRefusalText(blocked(EncounterBlockedReason.NO_USES), label)).toBe('');
  });

  it('splits the first sentence from the rest', () => {
    expect(leadOf('O Zumbi 1 saiu do alcance. Escolha outro alvo.')).toEqual({
      lead: 'O Zumbi 1 saiu do alcance.',
      rest: 'Escolha outro alvo.',
    });
    expect(leadOf('Sem mais')).toEqual({ lead: 'Sem mais', rest: '' });
  });
});
