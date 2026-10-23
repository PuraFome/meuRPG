import { create } from '@bufbuild/protobuf';

import {
  CharacterInTheDarkSchema,
  DarknessCause,
  ListCharactersInTheDarkResponseSchema,
} from '../../../gen/meurpg/maps/v1/maps_pb';
import {
  darkAnnouncements,
  darkCharacters,
  darkLine,
  darkNotice,
  darkSnapshot,
} from './in-the-dark';
import { mapToken } from './map-builders-testing';

const brisa = { characterId: 'c-brisa', name: 'Brisa', col: 3, row: 4 };
const jabuti = { characterId: 'c-jabuti', name: 'Jabuti', col: 9, row: 4 };

describe('the master’s light warning text', () => {
  it('says nothing for nobody: there is no empty notice', () => {
    expect(darkNotice([])).toBeNull();
    expect(darkLine([])).toBe('');
  });

  it('names the one character and what is missing', () => {
    expect(darkNotice([brisa])).toEqual({
      many: false,
      lead: 'No escuro:',
      text: 'Brisa não enxerga onde está (sem luz e sem visão no escuro).',
      detail: '',
    });
  });

  it('counts the characters when there are several, with the cause once', () => {
    expect(darkNotice([brisa, jabuti])).toEqual({
      many: true,
      lead: 'No escuro:',
      text: '2 personagens não enxergam onde estão.',
      detail: 'Sem luz e sem visão no escuro.',
    });
  });

  it('shrinks to one line of names for the painting mode', () => {
    expect(darkLine([brisa])).toBe('No escuro: Brisa.');
    expect(darkLine([brisa, jabuti])).toBe('No escuro: Brisa, Jabuti.');
  });
});

describe('what the server answers', () => {
  it('keeps the id, the name and the square of each character', () => {
    const res = create(ListCharactersInTheDarkResponseSchema, {
      characters: [
        create(CharacterInTheDarkSchema, {
          characterId: 'c-brisa',
          characterName: 'Brisa',
          col: 3,
          row: 4,
          cause: DarknessCause.NO_LIGHT_NO_DARKVISION,
        }),
      ],
    });
    expect(darkCharacters(res)).toEqual([brisa]);
  });
});

describe('the live messages (states 8 and 11)', () => {
  const tokens = (bilbo: string, brisaLight: string) => [
    mapToken('c-bilbo', 'bilbo', { carriedLight: bilbo }),
    mapToken('c-brisa', 'Brisa', { carriedLight: brisaLight }),
  ];

  it('says that a torch went out and what the light became', () => {
    const before = darkSnapshot(tokens('light:torch', ''), []);
    const after = darkSnapshot(tokens('', ''), []);
    expect(darkAnnouncements(before, after)).toEqual([
      'A tocha de bilbo apagou. A luz de bilbo ficou “Nenhuma”.',
    ]);
  });

  it('says that the one who was in the dark lit a light and is out of it', () => {
    const before = darkSnapshot(tokens('light:torch', ''), [brisa]);
    const after = darkSnapshot(tokens('light:torch', 'light:torch'), []);
    expect(darkAnnouncements(before, after)).toEqual([
      'Brisa acendeu a tocha. Não está mais no escuro.',
    ]);
  });

  it('names the other lights in their own words', () => {
    const before = darkSnapshot(tokens('light:hooded-lantern', ''), []);
    const after = darkSnapshot(tokens('', ''), []);
    expect(darkAnnouncements(before, after)[0]).toBe(
      'A lanterna coberta de bilbo apagou. A luz de bilbo ficou “Nenhuma”.',
    );
  });

  it('is silent when nothing changed, when the first read has nothing to compare, and when one left the dark without a light', () => {
    const same = darkSnapshot(tokens('light:torch', ''), [brisa]);
    expect(darkAnnouncements(same, same)).toEqual([]);
    expect(darkAnnouncements(null, same)).toEqual([]);
    const out = darkSnapshot(tokens('light:torch', ''), []);
    expect(darkAnnouncements(same, out)).toEqual([]);
  });
});
