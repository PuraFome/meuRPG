import { create } from '@bufbuild/protobuf';

import {
  ChoiceGroupSchema,
  ChoiceKind,
  ChoiceOptionSchema,
  ChoiceSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import {
  type ChoiceDrafts,
  doneCount,
  draftOf,
  keepDraftsOf,
  missingIn,
  storedKeysOf,
  textsOf,
  withSelection,
  withText,
} from './levelup-choices';

const opt = (key: string, needsText = false) =>
  create(ChoiceOptionSchema, { key, storedKey: `stored:${key}`, namePt: key, needsText });

const style = create(ChoiceSchema, {
  key: 'style',
  labelPt: 'Estilo de Luta (Guerreiro, nível 1)',
  kind: ChoiceKind.OPTIONS,
  picks: 1,
  missing: 1,
  options: [opt('a'), opt('b')],
});
const invocations = create(ChoiceSchema, {
  key: 'inv',
  labelPt: 'Invocações',
  kind: ChoiceKind.OPTIONS,
  picks: 2,
  missing: 1,
  picked: ['x'],
  options: [opt('x'), opt('y'), opt('z')],
});
const enemy = create(ChoiceSchema, {
  key: 'enemy',
  labelPt: 'Inimigo Favorito',
  kind: ChoiceKind.ENEMY,
  picks: 1,
  missing: 1,
  options: [opt('beast'), opt('humanoid', true)],
});
const groups = [create(ChoiceGroupSchema, { choices: [style, invocations, enemy] })];
const none: ChoiceDrafts = new Map();

describe('the late and new choices of a level-up', () => {
  it('counts what is still missing, by the label of each choice', () => {
    expect(missingIn(groups, none)).toEqual({
      count: 3,
      labels: ['Estilo de Luta (Guerreiro, nível 1)', 'Invocações', 'Inimigo Favorito'],
    });
    const some = withSelection(withSelection(none, style, ['a']), invocations, ['x', 'y']);
    expect(missingIn(groups, some)).toEqual({ count: 1, labels: ['Inimigo Favorito'] });
  });

  it('keeps a pick the sheet already has, whatever the card shows', () => {
    const unpicked = withSelection(none, invocations, []);
    expect(doneCount(invocations, draftOf(unpicked, invocations))).toBe(1);
  });

  it('counts a humanoid favored enemy one short until both races are written', () => {
    let d = withSelection(none, enemy, ['humanoid']);
    expect(doneCount(enemy, draftOf(d, enemy))).toBe(0);
    d = withText(withText(d, enemy, 1, 'Orcs'), enemy, 2, ' ');
    expect(doneCount(enemy, draftOf(d, enemy))).toBe(0);
    d = withText(d, enemy, 2, 'Goblins');
    expect(doneCount(enemy, draftOf(d, enemy))).toBe(1);
    expect(missingIn(groups, d).labels).not.toContain('Inimigo Favorito');
  });

  it('sends the stored key of what was picked now, never of what the sheet had', () => {
    const d = withSelection(withSelection(none, style, ['b']), invocations, ['x', 'z']);
    expect(storedKeysOf(groups, d)).toEqual(['stored:b', 'stored:z']);
    expect(storedKeysOf(groups, none)).toEqual([]);
  });

  it('sends the free texts by their key, only for an option that takes them', () => {
    let d = withSelection(none, enemy, ['humanoid']);
    d = withText(withText(d, enemy, 1, 'Orcs'), enemy, 2, 'Goblins');
    expect(textsOf(groups, d)).toEqual({ 'enemy#1': 'Orcs', 'enemy#2': 'Goblins' });
    const beast = withSelection(d, enemy, ['beast']);
    expect(textsOf(groups, beast)).toEqual({});
  });

  it('forgets the picks of a choice the sheet no longer asks, and of an option it no longer offers', () => {
    const d = withSelection(withSelection(none, style, ['a']), invocations, ['x', 'gone']);
    const kept = keepDraftsOf([create(ChoiceGroupSchema, { choices: [invocations] })], d);
    expect([...kept.keys()]).toEqual(['inv']);
    expect(kept.get('inv')?.optionKeys).toEqual(['x']);
  });
});
