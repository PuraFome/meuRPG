import { create } from '@bufbuild/protobuf';

import { AttackKind, AttackSchema } from '../../../gen/meurpg/rules/v1/rules_pb';
import { ammunitionEmptyLine, ammunitionNote, ammunitionTag } from './ammunition-line';

const line = (key: string, extra: Record<string, unknown> = {}) =>
  create(AttackSchema, { key, name: key, kind: AttackKind.WEAPON, ...extra });

const wrap = (...attacks: ReturnType<typeof line>[]) => attacks.map((attack) => ({ attack }));

const KEY_OF: Record<string, string> = {
  'equipment:shortbow': 'equipment:arrow',
  'equipment:longbow': 'equipment:arrow',
  'equipment:crossbow-light': 'equipment:crossbow-bolt',
  'equipment:crossbow-hand': 'equipment:crossbow-bolt',
  'equipment:crossbow-heavy': 'equipment:crossbow-bolt',
  'equipment:sling': 'equipment:sling-bullet',
  'equipment:blowgun': 'equipment:blowgun-needle',
};
/** A ranged line as the server sends it: it carries the key of its ammunition. */
const shot = (weapon: string, extra: Record<string, unknown> = {}) =>
  line(weapon, { ammunitionKey: KEY_OF[weapon] ?? '', ...extra });

describe('the ammunition on an attack row', () => {
  it('has no tag for a kind the app does not count (ammunition_left absent)', () => {
    expect(ammunitionTag(shot('equipment:longbow'))).toBeNull();
    expect(ammunitionEmptyLine(shot('equipment:longbow'))).toBe('');
  });

  it('counts the pieces with the name of the kind, in the singular for one', () => {
    expect(ammunitionTag(shot('equipment:longbow', { ammunitionLeft: 7 }))).toEqual({
      text: '7 flechas',
      level: 'ok',
    });
    expect(ammunitionTag(shot('equipment:longbow', { ammunitionLeft: 1 }))?.text).toBe('1 flecha');
  });

  it('names the kind by the ammunition key of the line: bow, crossbow, sling and blowgun', () => {
    const word = (key: string) => ammunitionTag(shot(key, { ammunitionLeft: 2 }))?.text;
    expect(word('equipment:shortbow')).toBe('2 flechas');
    expect(word('equipment:crossbow-light')).toBe('2 virotes');
    expect(word('equipment:crossbow-hand')).toBe('2 virotes');
    expect(word('equipment:crossbow-heavy')).toBe('2 virotes');
    expect(word('equipment:sling')).toBe('2 balas de funda');
    expect(word('equipment:blowgun')).toBe('2 agulhas de zarabatana');
    expect(ammunitionTag(shot('equipment:blowgun', { ammunitionLeft: 1 }))?.text).toBe(
      '1 agulha de zarabatana',
    );
    expect(ammunitionTag(shot('equipment:sling', { ammunitionLeft: 1 }))?.text).toBe(
      '1 bala de funda',
    );
  });

  it('says "munição" for a line whose ammunition key the app does not know', () => {
    expect(
      ammunitionTag(
        line('equipment:harpoon-gun', { ammunitionKey: 'equipment:harpoon', ammunitionLeft: 4 }),
      )?.text,
    ).toBe('4 munições');
    expect(
      ammunitionTag(
        line('equipment:harpoon-gun', { ammunitionKey: 'equipment:harpoon', ammunitionLeft: 1 }),
      )?.text,
    ).toBe('1 munição');
    expect(
      ammunitionEmptyLine(
        line('equipment:harpoon-gun', { ammunitionKey: 'equipment:harpoon', ammunitionLeft: 0 }),
      ),
    ).toBe('Sem munição: o tiro não sai.');
  });

  it('warns at 3 or fewer and is empty at 0', () => {
    const level = (n: number) =>
      ammunitionTag(shot('equipment:longbow', { ammunitionLeft: n }))?.level;
    expect(level(4)).toBe('ok');
    expect(level(3)).toBe('warning');
    expect(level(1)).toBe('warning');
    expect(level(0)).toBe('empty');
  });

  it('says why the shot does not go only at 0', () => {
    expect(ammunitionEmptyLine(shot('equipment:longbow', { ammunitionLeft: 0 }))).toBe(
      'Sem flechas: o tiro não sai.',
    );
    expect(ammunitionEmptyLine(shot('equipment:crossbow-light', { ammunitionLeft: 0 }))).toBe(
      'Sem virotes: o tiro não sai.',
    );
    expect(ammunitionEmptyLine(shot('equipment:longbow', { ammunitionLeft: 2 }))).toBe('');
  });

  it('notes the improvised melee line with the kind of its ranged sibling: it spends no piece, and nothing counts on it', () => {
    const melee = line('equipment:longbow:melee', { improvised: true });
    expect(
      ammunitionNote(melee, wrap(shot('equipment:longbow', { ammunitionLeft: 3 }), melee)),
    ).toBe('Improvisado: não gasta flecha.');
    expect(ammunitionTag(melee)).toBeNull();
    expect(ammunitionEmptyLine(melee)).toBe('');
    const crossbow = line('equipment:crossbow-light:melee', { improvised: true });
    expect(ammunitionNote(crossbow, wrap(shot('equipment:crossbow-light'), crossbow))).toBe(
      'Improvisado: não gasta virote.',
    );
    expect(ammunitionNote(shot('equipment:longbow', { ammunitionLeft: 3 }), wrap())).toBe('');
  });

  it('falls back to "munição" on the melee line when no ranged sibling is in the options', () => {
    const melee = line('equipment:longbow:melee', { improvised: true });
    expect(ammunitionNote(melee, wrap(melee))).toBe('Improvisado: não gasta munição.');
    expect(ammunitionNote(melee, wrap())).toBe('Improvisado: não gasta munição.');
    const other = shot('equipment:sling');
    expect(ammunitionNote(melee, wrap(other, melee))).toBe('Improvisado: não gasta munição.');
  });

  it('does not guess the kind from the weapon key', () => {
    expect(ammunitionTag(line('equipment:longbow', { ammunitionLeft: 5 }))?.text).toBe(
      '5 munições',
    );
  });
});
