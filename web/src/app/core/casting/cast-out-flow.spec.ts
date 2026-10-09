import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';

import {
  CastEffect,
  CastingEffectKind,
  CastingReachSchema,
  CastingSpellSchema,
  CastingTargetSchema,
  OutsideCastEnd,
  OutsideCastSchema,
  OutsideCastStatus,
  OutsideCastTargetSchema,
  RestThatEnds,
} from '../../../gen/meurpg/play/v1/casting_pb';
import { SpellSchema } from '../../../gen/meurpg/rules/v1/rules_pb';
import {
  activeChip,
  castButton,
  defaultWay,
  durationWords,
  endsConcentrationOf,
  endWords,
  isLong,
  lastsWords,
  logLine,
  minutesWords,
  namesList,
  needsTarget,
  resultSentences,
  resultTitle,
  ritualTimeText,
  rollLine,
  spellLine,
  targetHeading,
  targetRows,
  targetRuleOf,
  toggledTarget,
  waysOf,
} from './cast-out-flow';

/** The text with the non-breaking spaces the app keeps numbers and units together with written as plain spaces. */
const plain = (t: string): string => t.replace(/\u00a0/g, ' ');

const spell = (over: MessageInitShape<typeof CastingSpellSchema> = {}, level = 1, tag = {}) =>
  create(CastingSpellSchema, {
    spell: create(SpellSchema, { key: 'spell:x', level, ...tag }),
    canCast: true,
    castingTimePt: '1 ação',
    ...over,
  });

describe('game time in words', () => {
  it('says a duration in the largest whole unit, never as a clock time', () => {
    expect(durationWords(8 * 3600)).toBe('8 horas');
    expect(durationWords(3600)).toBe('1 hora');
    expect(durationWords(60)).toBe('1 minuto');
    expect(durationWords(600)).toBe('10 minutos');
    expect(durationWords(6)).toBe('1 rodada');
    expect(durationWords(86400)).toBe('1 dia');
    expect(durationWords(8 * 3600)).not.toMatch(/\d{1,2}[h:]\d{2}/);
  });

  it('says how long a casting takes', () => {
    expect(minutesWords(11)).toBe('11 minutos');
    expect(minutesWords(60)).toBe('1 hora');
    expect(minutesWords(70)).toBe('1 hora e 10 minutos');
  });
});

describe('the spell line', () => {
  it('writes the circle, the time and what lasts, as the boards draw them', () => {
    const bless = spell({ lasts: true, durationSeconds: 60 }, 1, { concentration: true });
    expect(plain(lastsWords(bless))).toBe('concentração, até 1 minuto');
    expect(plain(spellLine(bless))).toBe('1º nível · 1 ação · concentração, até 1 minuto');
    const aid = spell({ lasts: true, durationSeconds: 8 * 3600 }, 2);
    expect(plain(spellLine(aid))).toBe('2º nível · 1 ação · dura 8 horas');
  });

  it('shows the reach of a spell that lasts nothing, and the ritual tag', () => {
    const cure = spell({ rangeKind: 'touch' });
    expect(plain(spellLine(cure))).toBe('1º nível · 1 ação · toque');
    const detect = spell({ lasts: true, durationSeconds: 600, ritualAllowed: true }, 1, {
      concentration: true,
      ritual: true,
    });
    expect(plain(spellLine(detect))).toBe(
      '1º nível · 1 ação · concentração, até 10 minutos · ritual',
    );
    expect(spellLine(spell({}, 0))).toMatch(/^Truque/);
  });
});

describe('the ways to cast', () => {
  it('offers the ritual only to a class that can, and opens on the ritual when it is all there is', () => {
    const both = spell({ canCast: true, ritualAllowed: true, ritualMinutes: 10 });
    expect(waysOf(both)).toEqual(['slot', 'ritual']);
    const book = spell({
      canCast: false,
      ritualAllowed: true,
      ritualMinutes: 11,
      castingMinutes: 1,
    });
    expect(waysOf(book)).toEqual(['ritual']);
    expect(defaultWay(book)).toBe('ritual');
    expect(defaultWay(both)).toBe('slot');
    expect(waysOf(spell({ canCast: false }))).toEqual([]);
  });

  it('works the ritual time out of the spell: its casting time plus 10 minutes (SRD 5.1, Rituals)', () => {
    const detect = spell({
      ritualAllowed: true,
      ritualMinutes: 10,
      castingMinutes: 0,
      castingTimePt: '1 ação',
    });
    expect(ritualTimeText(detect)).toBe('1 ação + 10 minutos');
    const alarm = spell({
      ritualAllowed: true,
      ritualMinutes: 11,
      castingMinutes: 1,
      castingTimePt: '1 minuto',
    });
    expect(ritualTimeText(alarm)).toBe('1 minuto + 10 = 11 minutos');
    expect(isLong(detect, 'ritual')).toBe(true);
    expect(isLong(detect, 'slot')).toBe(false);
  });
});

describe('the targets', () => {
  const cure = spell({ maxTargets: 1, rangeKind: 'touch', effect: CastingEffectKind.HEAL });
  const aid = spell(
    { maxTargets: 3, targetsPerLevel: 3, effect: CastingEffectKind.MAX_HIT_POINTS },
    2,
  );

  it("takes one target with a radio and up to the spell's number with boxes, one more per level above", () => {
    expect(targetRuleOf(cure, 1, false)).toEqual({ kind: 'one', max: 1 });
    expect(targetRuleOf(aid, 2, false)).toEqual({ kind: 'many', max: 3 });
    expect(targetRuleOf(aid, 3, false)).toEqual({ kind: 'many', max: 6 });
    expect(targetRuleOf(spell({ casterOnly: true }), 1, false)).toEqual({ kind: 'none', max: 0 });
  });

  it('does not hold the master to the number', () => {
    expect(targetRuleOf(aid, 2, true).max).toBe(10);
  });

  it('toggles inside the rule', () => {
    const one = targetRuleOf(cure, 1, false);
    expect(toggledTarget(one, ['a'], 'b')).toEqual(['b']);
    expect(toggledTarget(one, ['a'], 'a')).toEqual([]);
    const three = targetRuleOf(aid, 2, false);
    expect(toggledTarget(three, ['a', 'b'], 'c')).toEqual(['a', 'b', 'c']);
    expect(toggledTarget(three, ['a', 'b', 'c'], 'd')).toEqual(['a', 'b', 'c']);
  });

  it('words the heading by the range', () => {
    expect(plain(targetHeading(cure, targetRuleOf(cure, 1, false)))).toBe(
      'Quem você toca (uma criatura, até 1,5 m)',
    );
    expect(plain(targetHeading(aid, targetRuleOf(aid, 2, false)))).toBe(
      'Quem recebe (até 3 criaturas)',
    );
    const far = spell({ maxTargets: 1, rangeKind: 'ranged', rangeFt: 30 });
    expect(plain(targetHeading(far, targetRuleOf(far, 1, false)))).toBe(
      'Quem recebe (uma criatura, até 9 m)',
    );
  });

  it("lists who is in reach and says why not, never anything about an NPC's health", () => {
    const withReach = spell({
      maxTargets: 1,
      reach: [
        create(CastingReachSchema, { characterId: 'toren', inRange: true }),
        create(CastingReachSchema, {
          characterId: 'brisa',
          inRange: false,
          reasonPt: 'Fora do alcance do toque (1,5 m).',
        }),
        create(CastingReachSchema, { characterId: 'vesna', inRange: true }),
      ],
    });
    const rows = targetRows(
      withReach,
      [
        create(CastingTargetSchema, {
          characterId: 'toren',
          name: 'Toren',
          distanceKnown: true,
          distanceFt: 5,
        }),
        create(CastingTargetSchema, {
          characterId: 'brisa',
          name: 'Brisa',
          distanceKnown: true,
          distanceFt: 15,
        }),
        create(CastingTargetSchema, { characterId: 'vesna', name: 'Vesna, a capitã', npc: true }),
      ],
      'ilaria',
    );
    expect(rows.map((r) => [r.name, r.enabled, plain(r.detail), plain(r.reason)])).toEqual([
      ['Toren', true, 'a 1,5 m', ''],
      ['Brisa', false, 'a 4,5 m', 'Fora do alcance do toque (1,5 m).'],
      ['Vesna, a capitã', true, 'NPC que o mestre mostrou', ''],
    ]);
    expect(JSON.stringify(rows)).not.toMatch(/ferid|PV de/i);
  });

  it('asks for a target only when the server applies something to someone', () => {
    expect(needsTarget(cure)).toBe(true);
    expect(needsTarget(spell({ effect: CastingEffectKind.NARRATED }))).toBe(false);
    expect(
      needsTarget(spell({ casterOnly: true, effect: CastingEffectKind.TEMPORARY_HIT_POINTS })),
    ).toBe(false);
  });

  it('writes the button of the step', () => {
    const toren = targetRows(
      cure,
      [create(CastingTargetSchema, { characterId: 'toren', name: 'Toren' })],
      'x',
    );
    expect(castButton('Curar Ferimentos', 'slot', false, toren, true)).toBe(
      'Curar Ferimentos em Toren',
    );
    expect(castButton('Alarme', 'ritual', true, [])).toBe('Começar o ritual');
    expect(castButton('Prece de Cura', 'slot', true, toren)).toBe('Começar a conjuração');
    expect(castButton('Luz', 'slot', false, [])).toBe('Conjurar Luz');
    expect(castButton('Ajuda', 'slot', false, toren)).toBe('Conjurar Ajuda em Toren');
  });
});

describe('what a cast did', () => {
  const heal = (extra: number) =>
    create(OutsideCastSchema, {
      spellNamePt: 'Curar Ferimentos',
      diceCount: 1,
      diceSides: 8,
      faces: [6],
      rollTotal: 9,
      targets: [
        create(OutsideCastTargetSchema, {
          name: 'Toren',
          effect: CastEffect.HEAL,
          amount: 12,
          extraHitPoints: extra,
        }),
      ],
    });

  it('writes the healing line with the Life Domain extra', () => {
    expect(rollLine(heal(3), 3)).toBe(
      '1d8 (6) + 3 (modificador) + 3 (Discípulo da Vida: 2 + nível da magia) = 12',
    );
    expect(rollLine(heal(0), 0)).toBe('1d8 (6) + 3 (modificador) = 9');
    expect(resultTitle(heal(3))).toBe('Toren foi curado');
  });

  it('lists the people reached, and says a cast the server only records is recorded', () => {
    expect(namesList(['Toren', 'Brisa', 'Kai'])).toBe('Toren, Brisa e Kai');
    expect(namesList(['Toren'])).toBe('Toren');
    expect(resultSentences(heal(3))).toEqual(['Toren recuperou 12 PV.']);
    const narrated = create(OutsideCastSchema, { spellNamePt: 'Detectar Magia' });
    expect(resultSentences(narrated)[0]).toMatch(/registrada/);
    expect(resultTitle(narrated)).toBe('Detectar Magia conjurada');
  });

  it('does not repeat what a player may not read: a target with no amount has no number in its sentence', () => {
    const hidden = create(OutsideCastSchema, {
      targets: [create(OutsideCastTargetSchema, { name: 'Toren', effect: CastEffect.HEAL })],
    });
    expect(resultSentences(hidden)).toEqual(['Toren foi curado.']);
  });

  it('words Mage Armor', () => {
    const armor = create(OutsideCastSchema, {
      spellNamePt: 'Armadura Arcana',
      targets: [
        create(OutsideCastTargetSchema, {
          name: 'Pensantus',
          effect: CastEffect.ARMOR_CLASS,
          armorClass: 15,
        }),
      ],
    });
    expect(resultTitle(armor)).toBe('Armadura Arcana em Pensantus');
    expect(resultSentences(armor)).toEqual(['Pensantus fica com CA 13 + Destreza (15).']);
  });
});

describe('the casts that go on', () => {
  it('shows the end of a spell that lasts in game time', () => {
    const aid = create(OutsideCastSchema, {
      status: OutsideCastStatus.ACTIVE,
      durationSeconds: 8 * 3600,
    });
    expect(activeChip(aid)).toBe('dura 8 horas');
    const bless = create(OutsideCastSchema, {
      status: OutsideCastStatus.ACTIVE,
      durationSeconds: 60,
      concentrating: true,
    });
    expect(activeChip(bless)).toBe('concentração · até 1 minuto');
  });

  it('says why a cast is over', () => {
    expect(endWords(create(OutsideCastSchema, { endReason: OutsideCastEnd.INTERRUPTED }))).toMatch(
      /espaço não foi gasto/,
    );
    expect(endWords(create(OutsideCastSchema, { endReason: OutsideCastEnd.REST }))).toMatch(
      /descanso/,
    );
  });

  it('asks before a second concentration and says which it ends', () => {
    const bless = create(OutsideCastSchema, { spellNamePt: 'Bênção', concentrating: true });
    const detect = spell({}, 1, { concentration: true });
    expect(endsConcentrationOf(detect, 'slot', bless)).toBe('Bênção');
    expect(endsConcentrationOf(spell({}, 1), 'slot', bless)).toBeNull();
    // A casting that takes time is concentration from the start (SRD 5.1, Longer Casting Times).
    const alarm = spell({ ritualMinutes: 11, ritualAllowed: true });
    expect(endsConcentrationOf(alarm, 'ritual', bless)).toBe('Bênção');
    expect(endsConcentrationOf(detect, 'slot', undefined)).toBeNull();
  });

  it('names the rest that ends a spell', () => {
    expect(RestThatEnds.LONG).toBeDefined();
  });
});

describe('logLine', () => {
  const base = { spellNamePt: 'Curar Ferimentos', casterName: 'Ilaria' };
  it('says who cast what on whom', () => {
    const cast = create(OutsideCastSchema, {
      ...base,
      status: OutsideCastStatus.ENDED,
      targets: [{ name: 'Toren' }, { name: 'Brisa' }],
    });
    expect(plain(logLine(cast))).toBe('Ilaria conjurou Curar Ferimentos em Toren e Brisa.');
  });
  it('says a long cast is being cast, and a failed one spent nothing', () => {
    const going = create(OutsideCastSchema, {
      ...base,
      status: OutsideCastStatus.CASTING,
      ritual: true,
    });
    expect(logLine(going)).toBe('Ilaria está conjurando Curar Ferimentos como ritual.');
    const failed = create(OutsideCastSchema, {
      ...base,
      status: OutsideCastStatus.FAILED,
      endReason: OutsideCastEnd.INTERRUPTED,
    });
    expect(logLine(failed)).toContain('falhou');
    expect(logLine(failed)).toContain('espaço não foi gasto');
  });
});
