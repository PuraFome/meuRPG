import { type MessageInitShape, create } from '@bufbuild/protobuf';

import {
  CombatLogEffectChange,
  CombatLogEffectSchema,
  type CombatLogEntry,
  CombatLogEntrySchema,
  CombatLogKind,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { LapseNotices, lapseNoticeText } from './lapse-notice';

const clean = (s: string) => s.replace(/ /g, ' ');

const ended = (
  id: string,
  fx: MessageInitShape<typeof CombatLogEffectSchema> = {},
): CombatLogEntry =>
  create(CombatLogEntrySchema, {
    id,
    kind: CombatLogKind.EFFECT,
    effect: create(CombatLogEffectSchema, {
      sourceKey: 'spell:branding-smite',
      sourceNamePt: 'Clarão na Lâmina',
      targetLabels: ['Brisa'],
      change: CombatLogEffectChange.ENDED,
      reason: 'concentration',
      unused: true,
      ...fx,
    }),
  });

const viewer = (over: { master?: boolean; ownLabel?: string } = {}) => ({
  master: false,
  ownLabel: 'Brisa',
  players: new Set(['Brisa']),
  ...over,
});

describe('lapseNoticeText', () => {
  it('tells the caster that the concentration ended and the effect with it, with no hit', () => {
    expect(clean(lapseNoticeText({ spell: 'Clarão na Lâmina', caster: 'Brisa' }, viewer()))).toBe(
      'A sua concentração acabou. O Clarão na Lâmina acabou sem golpe.',
    );
  });

  it('tells the master whose concentration it was', () => {
    expect(
      clean(
        lapseNoticeText(
          { spell: 'Clarão na Lâmina', caster: 'Brisa' },
          viewer({ master: true, ownLabel: '' }),
        ),
      ),
    ).toBe('A concentração de Brisa acabou. O Clarão na Lâmina de Brisa acabou sem golpe.');
  });

  it('gives a creature caster its article for the master', () => {
    expect(
      clean(
        lapseNoticeText(
          { spell: 'Clarão na Lâmina', caster: 'Goblin 1' },
          viewer({ master: true, ownLabel: '' }),
        ),
      ),
    ).toBe('A concentração do Goblin 1 acabou. O Clarão na Lâmina do Goblin 1 acabou sem golpe.');
  });
});

describe('LapseNotices', () => {
  const read = (n: LapseNotices, entries: CombatLogEntry[], v = viewer(), enc = 'enc') =>
    n.read(enc, true, entries, v);

  it('does not tell what the log already held when it was first read', () => {
    const n = new LapseNotices();
    expect(read(n, [ended('a')]).notice).toBeNull();
    expect(read(n, [ended('a')]).notice).toBeNull();
  });

  it('tells the caster a new unused end by concentration, once', () => {
    const n = new LapseNotices();
    read(n, []);
    expect(read(n, [ended('a')]).notice).toEqual({ spell: 'Clarão na Lâmina', caster: 'Brisa' });
    expect(read(n, [ended('a')]).notice).toBeNull();
  });

  it('tells the master, whoever the caster is', () => {
    const n = new LapseNotices();
    const m = viewer({ master: true, ownLabel: '' });
    read(n, [], m);
    expect(read(n, [ended('a', { targetLabels: ['Goblin 1'] })], m).notice).toEqual({
      spell: 'Clarão na Lâmina',
      caster: 'Goblin 1',
    });
  });

  it('says nothing to another player: the log line is theirs', () => {
    const n = new LapseNotices();
    const other = viewer({ ownLabel: 'Tor' });
    read(n, [], other);
    expect(read(n, [ended('a')], other).notice).toBeNull();
  });

  it('says nothing for an effect a hit spent, one that ended for another reason, or another change', () => {
    const n = new LapseNotices();
    read(n, []);
    for (const [i, over] of [
      { unused: false },
      { reason: 'duration' },
      { reason: 'ended_by_master' },
      { change: CombatLogEffectChange.ADDED },
      { targetLabels: [] },
    ].entries()) {
      expect(read(n, [ended(`x${i}`, over)]).notice).toBeNull();
    }
  });

  it('waits for the log to be loaded and a viewer to be known, and starts clean in another combat', () => {
    const n = new LapseNotices();
    expect(n.read('enc', false, [ended('a')], viewer()).notice).toBeNull();
    expect(read(n, [ended('a')]).notice).toBeNull(); // first loaded read
    expect(read(n, [ended('a'), ended('b')]).notice).not.toBeNull();
    const out = read(n, [ended('a')], viewer(), 'other');
    expect(out.reset).toBe(true);
    expect(out.notice).toBeNull();
  });
});
