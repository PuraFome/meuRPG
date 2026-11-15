import { create } from '@bufbuild/protobuf';

import { AttackKind, AttackSchema } from '../../../gen/meurpg/rules/v1/rules_pb';
import { ammunitionEmptyLine, ammunitionNote, ammunitionTag } from './ammunition-line';

const line = (key: string, extra: Record<string, unknown> = {}) =>
  create(AttackSchema, { key, name: key, kind: AttackKind.WEAPON, ...extra });

describe('the ammunition on an attack row', () => {
  it('has no tag for a kind the app does not count (ammunition_left absent)', () => {
    expect(ammunitionTag(line('equipment:longbow'))).toBeNull();
    expect(ammunitionEmptyLine(line('equipment:longbow'))).toBe('');
  });

  it('counts the pieces with the name of the kind, in the singular for one', () => {
    expect(ammunitionTag(line('equipment:longbow', { ammunitionLeft: 7 }))).toEqual({
      text: '7 flechas',
      level: 'ok',
    });
    expect(ammunitionTag(line('equipment:longbow', { ammunitionLeft: 1 }))?.text).toBe('1 flecha');
  });

  it('names the kind by the weapon: bow, crossbow, sling and blowgun', () => {
    const word = (key: string) => ammunitionTag(line(key, { ammunitionLeft: 2 }))?.text;
    expect(word('equipment:shortbow')).toBe('2 flechas');
    expect(word('equipment:light-crossbow')).toBe('2 virotes');
    expect(word('equipment:hand-crossbow')).toBe('2 virotes');
    expect(word('equipment:heavy-crossbow')).toBe('2 virotes');
    expect(word('equipment:sling')).toBe('2 balas de funda');
    expect(word('equipment:blowgun')).toBe('2 agulhas de zarabatana');
    expect(ammunitionTag(line('equipment:blowgun', { ammunitionLeft: 1 }))?.text).toBe(
      '1 agulha de zarabatana',
    );
    expect(ammunitionTag(line('equipment:sling', { ammunitionLeft: 1 }))?.text).toBe(
      '1 bala de funda',
    );
  });

  it('says "munição" for a weapon the app cannot name the kind of', () => {
    expect(ammunitionTag(line('equipment:harpoon-gun', { ammunitionLeft: 4 }))?.text).toBe(
      '4 de munição',
    );
    expect(ammunitionEmptyLine(line('equipment:harpoon-gun', { ammunitionLeft: 0 }))).toBe(
      'Sem munição: o tiro não sai.',
    );
  });

  it('warns at 3 or fewer and is empty at 0', () => {
    const level = (n: number) =>
      ammunitionTag(line('equipment:longbow', { ammunitionLeft: n }))?.level;
    expect(level(4)).toBe('ok');
    expect(level(3)).toBe('warning');
    expect(level(1)).toBe('warning');
    expect(level(0)).toBe('empty');
  });

  it('says why the shot does not go only at 0', () => {
    expect(ammunitionEmptyLine(line('equipment:longbow', { ammunitionLeft: 0 }))).toBe(
      'Sem flechas: o tiro não sai.',
    );
    expect(ammunitionEmptyLine(line('equipment:light-crossbow', { ammunitionLeft: 0 }))).toBe(
      'Sem virotes: o tiro não sai.',
    );
    expect(ammunitionEmptyLine(line('equipment:longbow', { ammunitionLeft: 2 }))).toBe('');
  });

  it('notes the improvised melee line: it spends no piece, and nothing counts on it', () => {
    const melee = line('equipment:longbow:melee', { improvised: true });
    expect(ammunitionNote(melee)).toBe('Improvisado: não gasta flecha.');
    expect(ammunitionTag(melee)).toBeNull();
    expect(ammunitionEmptyLine(melee)).toBe('');
    expect(ammunitionNote(line('equipment:light-crossbow:melee', { improvised: true }))).toBe(
      'Improvisado: não gasta virote.',
    );
    expect(ammunitionNote(line('equipment:harpoon-gun:melee', { improvised: true }))).toBe(
      'Improvisado: não gasta munição.',
    );
    expect(ammunitionNote(line('equipment:longbow', { ammunitionLeft: 3 }))).toBe('');
  });
});
