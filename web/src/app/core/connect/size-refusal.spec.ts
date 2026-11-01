import { Code, ConnectError } from '@connectrpc/connect';

import { bestiaryErrorMessage } from '../creatures/bestiary-errors';
import { addMonstersErrorMessage } from '../combat/monsters';
import { describeCharacterError } from '../characters/character-errors';
import { sendErrorMessage } from '../conversation/conversation-errors';
import { generateIssue, useIssue } from '../images/imagegen-errors';
import { showImageIssue } from '../images/shown-client';
import {
  editorErrorMessage,
  mapErrorMessage,
  sceneActionErrorMessage,
  sceneClueErrorMessage,
} from '../maps/map-errors';
import {
  createFailure,
  placeSceneFailure,
  previewFailure,
  redrawFailure,
} from '../maps/dungeon-errors';
import { noteErrorMessage } from '../notes/notes-errors';
import { xpErrorMessage } from '../progression/xp-errors';
import { placeFailure } from '../treasure/treasure-errors';
import { REFUSED_BY_A_LIMIT } from './connect-errors';

/**
 * Every mapper that words `resource_exhausted` for its own quota (a full
 * gallery, 20 actions, 300 notes) says the generic limit text for a body over
 * its size limit instead. The server sends both with the same code, and the
 * two wordings of the size one are pinned in backend/internal/platform/reqlimit.
 */
const SIZE = new ConnectError(
  'read first 65536 bytes of message: exceeded 65536 byte http.MaxBytesReader limit',
  Code.ResourceExhausted,
);
const BACKSTOP = new ConnectError(
  'message size 4194305 is larger than configured max 4194304',
  Code.ResourceExhausted,
);
const QUOTA = new ConnectError('the scene already has 20 actions', Code.ResourceExhausted);

const MAPPERS: readonly (readonly [string, (err: unknown) => string])[] = [
  ['mapErrorMessage', (e) => mapErrorMessage(e)],
  ['sceneActionErrorMessage', (e) => sceneActionErrorMessage(e)],
  ['sceneClueErrorMessage', (e) => sceneClueErrorMessage(e)],
  ['editorErrorMessage (image)', (e) => editorErrorMessage(e, 'image', 'trocar a imagem')],
  ['editorErrorMessage (fog)', (e) => editorErrorMessage(e, 'fog', 'ligar a névoa')],
  ['editorErrorMessage (pointNew)', (e) => editorErrorMessage(e, 'pointNew', 'criar o ponto')],
  ['createFailure', (e) => createFailure(e).text],
  ['redrawFailure', (e) => redrawFailure(e)],
  ['placeSceneFailure', (e) => placeSceneFailure(e)],
  ['bestiaryErrorMessage', (e) => bestiaryErrorMessage(e, 'create')],
  ['addMonstersErrorMessage', (e) => addMonstersErrorMessage(e)],
  ['placeFailure (treasure)', (e) => placeFailure(e).text],
  ['generateIssue', (e) => generateIssue(e).text],
  ['useIssue', (e) => useIssue(e)],
  ['showImageIssue', (e) => showImageIssue(e)],
  ['xpErrorMessage', (e) => xpErrorMessage(e)],
  ['describeCharacterError', (e) => describeCharacterError(e)],
  ['noteErrorMessage', (e) => noteErrorMessage(e)],
  ['sendErrorMessage', (e) => sendErrorMessage(e)],
];

describe('a body over its size limit, on every screen with its own resource_exhausted text', () => {
  for (const [name, say] of MAPPERS) {
    it(`${name}: the generic limit text for a size refusal`, () => {
      expect(say(SIZE)).toContain(REFUSED_BY_A_LIMIT);
      expect(say(BACKSTOP)).toContain(REFUSED_BY_A_LIMIT);
    });

    it(`${name}: its own text for a quota refusal`, () => {
      expect(say(QUOTA)).not.toContain(REFUSED_BY_A_LIMIT);
    });
  }

  it('previewFailure: a size refusal is a failure with the generic text, not "busy"', () => {
    expect(previewFailure(SIZE)).toEqual({ kind: 'failed', text: REFUSED_BY_A_LIMIT });
    expect(previewFailure(QUOTA)).toEqual({ kind: 'busy' });
  });
});
