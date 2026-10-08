import { create } from '@bufbuild/protobuf';

import {
  CombatLogEntrySchema,
  CombatLogKind,
  CombatLogWildShapeSchema,
  CombatantKind,
  WildShapeEndReason,
} from '../../../gen/meurpg/play/v1/combat_pb';
import {
  ActionOptionSchema,
  DisabledReasonCode,
  DisabledReasonSchema,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import {
  ConcentrationWatch,
  FormNotices,
  formNoticeText,
  lostNoticeText,
  wildActionLine,
} from './combat-notices';
import { combatant } from './combat-testing';

const line = (
  id: string,
  over: { started?: boolean; reason?: WildShapeEndReason; carried?: number; actor?: string } = {},
) =>
  create(CombatLogEntrySchema, {
    id,
    kind: CombatLogKind.WILD_SHAPE,
    actorId: over.actor ?? 'sal',
    wildShape: create(CombatLogWildShapeSchema, {
      beastKey: 'monster:wolf',
      beastNamePt: 'Lobo',
      started: over.started ?? false,
      endReason: over.reason ?? WildShapeEndReason.DAMAGE,
      carriedDamage: over.carried ?? 0,
    }),
  });

const plain = (s: string) => s.replace(/\u00a0/g, ' ');

describe("the Wild Shape notice, from the combat log's line", () => {
  it('says the number the server carried, and only for the damage reason', () => {
    expect(
      plain(formNoticeText({ beast: 'Lobo', reason: WildShapeEndReason.DAMAGE, carried: 6 })),
    ).toBe('O Lobo caiu a 0 PV e você voltou à forma normal. 6 de dano passaram para você.');
    expect(
      plain(formNoticeText({ beast: 'Lobo', reason: WildShapeEndReason.DAMAGE, carried: 0 })),
    ).toBe('O Lobo caiu a 0 PV e você voltou à forma normal.');
  });

  it('says "caiu a 0 PV" only when that is the reason: the others have their own words', () => {
    expect(formNoticeText({ beast: 'Lobo', reason: WildShapeEndReason.MASTER, carried: 0 })).toBe(
      'O mestre levou o Lobo a 0 PV e você voltou à forma normal.',
    );
    expect(formNoticeText({ beast: 'Lobo', reason: WildShapeEndReason.ZERO_HP, carried: 0 })).toBe(
      'Você caiu a 0 PV e voltou à forma normal.',
    );
    expect(
      formNoticeText({ beast: 'Lobo', reason: WildShapeEndReason.UNCONSCIOUS, carried: 0 }),
    ).toBe('Você ficou inconsciente e voltou à forma normal.');
    for (const reason of [
      WildShapeEndReason.MASTER,
      WildShapeEndReason.ZERO_HP,
      WildShapeEndReason.UNCONSCIOUS,
    ]) {
      expect(formNoticeText({ beast: 'Lobo', reason, carried: 4 })).not.toContain('de dano');
    }
  });

  it('what the log already held is not news; a new end is, once', () => {
    const notices = new FormNotices();
    const old = [line('a', { carried: 3 })];
    // The first read of a combat (a reconnect, a page that opens late) only remembers.
    expect(notices.read('enc', 'sal', true, old)).toEqual({ reset: true, notice: null });
    expect(notices.read('enc', 'sal', true, old).notice).toBeNull();
    const fresh = [...old, line('b', { carried: 6 })];
    expect(notices.read('enc', 'sal', true, fresh).notice).toEqual({
      beast: 'Lobo',
      reason: WildShapeEndReason.DAMAGE,
      carried: 6,
    });
    // A re-read of the same log says nothing more.
    expect(notices.read('enc', 'sal', true, fresh).notice).toBeNull();
  });

  it("a form the druid left by choice, a start and another one's end say nothing", () => {
    const notices = new FormNotices();
    notices.read('enc', 'sal', true, []);
    expect(
      notices.read('enc', 'sal', true, [line('a', { reason: WildShapeEndReason.LEFT })]).notice,
    ).toBeNull();
    expect(notices.read('enc', 'sal', true, [line('b', { started: true })]).notice).toBeNull();
    expect(notices.read('enc', 'sal', true, [line('c', { actor: 'other' })]).notice).toBeNull();
  });

  it('another combat starts clean, and nothing is read before the log is', () => {
    const notices = new FormNotices();
    expect(notices.read('one', 'sal', false, [line('a')]).notice).toBeNull();
    notices.read('one', 'sal', true, []);
    expect(notices.read('two', 'sal2', true, [line('x')])).toEqual({ reset: true, notice: null });
  });
});

describe('the concentration notice', () => {
  const wolf = (n: number) =>
    combatant({
      id: `w${n}`,
      label: `Lobo atroz ${n}`,
      kind: CombatantKind.CREATURE,
      monsterKey: 'monster:dire-wolf',
      monsterNamePt: 'Lobo atroz',
      summonGroupId: 'cast',
    });
  const lost = (spell: string, gone = [wolf(1), wolf(2)]) => ({ spell, gone });

  it('is told when the spell goes with its creatures, and says which went', () => {
    const watch = new ConcentrationWatch();
    expect(watch.read('enc', 'Conjurar Animais', [wolf(1), wolf(2)], null)).toBeUndefined();
    const out = watch.read('enc', '', [], null);
    expect(out).toMatchObject({ spell: 'Conjurar Animais' });
    expect(plain(lostNoticeText(out!))).toBe(
      'Você perdeu a concentração em Conjurar Animais. Os 2 Lobos atrozes sumiram.',
    );
  });

  it('is not told when the player ended it, or when only another concentration took its place', () => {
    const watch = new ConcentrationWatch();
    watch.read('enc', 'Conjurar Animais', [wolf(1)], null);
    watch.endedByMe = true;
    expect(watch.read('enc', '', [], null)).toBeUndefined();
    watch.read('enc', 'Teia', [], null);
    expect(watch.read('enc', 'Sono', [], null)).toBeUndefined();
  });

  it('says "As" for a group of feminine creatures', () => {
    const spider = (n: number) =>
      combatant({
        id: `s${n}`,
        label: `Aranha gigante ${n}`,
        kind: CombatantKind.CREATURE,
        monsterKey: 'monster:giant-spider',
        monsterNamePt: 'Aranha gigante',
        summonGroupId: 'cast',
      });
    expect(plain(lostNoticeText(lost('Conjurar Animais', [spider(1), spider(2)])))).toMatch(
      / As 2 /,
    );
  });

  it('takes the notice away when the same spell is held again, even with no creature to come back', () => {
    const watch = new ConcentrationWatch();
    watch.read('enc', 'Teia', [], null);
    expect(watch.read('enc', 'Teia', [], lost('Teia', []))).toBeNull();
    expect(watch.read('enc', 'Sono', [], lost('Teia', []))).toBeUndefined();
  });

  it('an undo that brings the creatures back takes it away, and another combat never inherits it', () => {
    const watch = new ConcentrationWatch();
    watch.read('enc', 'Conjurar Animais', [wolf(1), wolf(2)], null);
    const shown = watch.read('enc', '', [], null)!;
    expect(watch.read('enc', 'Conjurar Animais', [wolf(1), wolf(2)], shown)).toBeNull();
    expect(watch.read('other', '', [], lost('Conjurar Animais'))).toBeNull();
    // A new combat, no spell: the held one of the old combat is not read as lost in the new.
    const w2 = new ConcentrationWatch();
    w2.read('one', 'Teia', [], null);
    expect(w2.read('two', '', [], null)).toBeUndefined();
  });
});

describe('the Wild Shape line of the turn', () => {
  const feature = (over: { enabled: boolean; code?: DisabledReasonCode; usesLeft?: number }) =>
    create(ActionOptionSchema, {
      action: { key: 'feature:wild-shape', namePt: 'Forma Selvagem', resourceKey: 'wild_shape' },
      enabled: over.enabled,
      usesLeft: over.usesLeft ?? 2,
      reason:
        over.code === undefined ? undefined : create(DisabledReasonSchema, { code: over.code }),
    });
  const resource = { key: 'wild_shape', total: 2, used: 0, recharge: 'short_rest' as const };

  it('says the uses and when they come back', () => {
    const line = wildActionLine(feature({ enabled: true }), false, resource);
    expect(line?.key).toBe('feature:wild-shape');
    expect(plain(line?.detail ?? '')).toBe(
      'Vire uma fera · restam 2 de 2 usos · volta no descanso curto ou longo',
    );
    expect(line?.reason).toBe('');
  });

  it('maps the server\'s reasons to "Sem usos" and "Sem ação disponível"', () => {
    expect(
      wildActionLine(
        feature({ enabled: false, code: DisabledReasonCode.NO_USES, usesLeft: 0 }),
        false,
        resource,
      )?.reason,
    ).toBe('Sem usos');
    expect(
      wildActionLine(
        feature({ enabled: false, code: DisabledReasonCode.ACTION_USED }),
        false,
        resource,
      )?.reason,
    ).toBe('Sem ação disponível');
  });

  it('is not there without the feature or while in a form', () => {
    expect(wildActionLine(undefined, false, resource)).toBeNull();
    expect(wildActionLine(feature({ enabled: true }), true, resource)).toBeNull();
  });
});
