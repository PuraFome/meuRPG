import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';
import { createRouterTransport } from '@connectrpc/connect';

import {
  CharacterKind,
  CharacterService,
  CharacterState,
  CharacterSummarySchema,
  ListCharactersResponseSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';
import { StageRoster } from './stage-roster';

describe('StageRoster', () => {
  it('lists the living NPCs from ONE ListCharacters call, with the portrait from the row (no sheet is read)', async () => {
    const calls: string[] = [];
    const row = (id: string, name: string, kind: CharacterKind, state: CharacterState, portraitUrl = '') =>
      create(CharacterSummarySchema, { id, name, kind, state, portraitUrl });
    const transport = createRouterTransport(({ service }) => {
      service(CharacterService, {
        listCharacters: () => {
          calls.push('list');
          return create(ListCharactersResponseSchema, {
            characters: [
              row('p', 'Pensantus', CharacterKind.PLAYER, CharacterState.LOCKED),
              row('m', 'Mira', CharacterKind.STORY, CharacterState.DRAFT, '/images/i1'),
              row('c', 'Capitão Goblin', CharacterKind.ENEMY, CharacterState.DRAFT),
              row('d', 'Morto', CharacterKind.MINION, CharacterState.DEAD),
            ],
          });
        },
        getCharacter: () => {
          calls.push('get');
          throw new Error('a sheet must not be read');
        },
      });
    });
    TestBed.configureTestingModule({ providers: [{ provide: CONNECT_TRANSPORT, useValue: transport }] });
    const list = await TestBed.inject(StageRoster).list('c1');
    expect(list).toEqual([
      { characterId: 'm', name: 'Mira', kindLabel: 'NPC de história', portraitUrl: '/images/i1/thumb' },
      { characterId: 'c', name: 'Capitão Goblin', kindLabel: 'Inimigo', portraitUrl: '' },
    ]);
    expect(calls).toEqual(['list']);
  });
});
