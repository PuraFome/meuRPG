import { SessionMessageSpeakerKind as Kind } from '../../../gen/meurpg/play/v1/session_messages_pb';
import { line } from './conversation-testing';
import {
  MAX_TEXT,
  buildSendRequest,
  chooserLine,
  joinNames,
  readersSummary,
  viewLine,
} from './message-view';

const directory = new Map([
  ['u-pen', 'Pensantus'],
  ['u-bri', 'Brisa'],
  ['u-tor', 'Toren'],
]);
const player = { userId: 'u-tor', isMaster: false };
const master = { userId: 'u-mestre', isMaster: true };

describe('viewLine: how a line looks to its reader', () => {
  it('narration is a plain "Narração" with no bubble', () => {
    const v = viewLine(line('1', { speakerKind: Kind.NARRATOR }), player, directory);
    expect(v).toMatchObject({ look: 'narration', label: 'Narração', audience: null });
  });

  it('an NPC line shows its name and an initial when it has no portrait', () => {
    const msg = line('1', { speakerKind: Kind.NPC, speakerId: 'n1', speakerName: 'Bartolo' });
    const v = viewLine(msg, player, directory);
    expect(v).toMatchObject({ look: 'npc', label: 'Bartolo', initial: 'B', portraitUrl: null });
  });

  it('an NPC portrait is the thumbnail', () => {
    const msg = line('1', {
      speakerKind: Kind.NPC,
      speakerName: 'Bartolo',
      speakerPortrait: { id: 'i', url: '/images/i', thumbnailUrl: '/images/i/thumb' },
    });
    expect(viewLine(msg, player, directory).portraitUrl).toBe('/images/i/thumb');
  });

  it("the player's own character line is '<name> · você'", () => {
    const msg = line('1', {
      speakerKind: Kind.CHARACTER,
      speakerName: 'Toren',
      authorUserId: 'u-tor',
    });
    expect(viewLine(msg, player, directory)).toMatchObject({
      look: 'own',
      label: 'Toren · você',
    });
  });

  it("another player's character line looks like a spoken line with the name", () => {
    const msg = line('1', {
      speakerKind: Kind.CHARACTER,
      speakerName: 'Brisa',
      authorUserId: 'u-bri',
    });
    expect(viewLine(msg, player, directory)).toMatchObject({ look: 'npc', label: 'Brisa' });
  });

  it("the master's own NPC line reads '<name> · você' like the board", () => {
    const msg = line('1', {
      speakerKind: Kind.NPC,
      speakerName: 'Bartolo, o taverneiro',
      authorUserId: 'u-mestre',
    });
    expect(viewLine(msg, master, directory)).toMatchObject({
      look: 'own',
      label: 'Bartolo, o taverneiro · você',
    });
  });

  it('out of character is muted and names the player', () => {
    const msg = line('1', {
      speakerKind: Kind.OUT_OF_CHARACTER,
      speakerName: 'Celso',
      authorUserId: 'u-cel',
    });
    expect(viewLine(msg, player, directory)).toMatchObject({
      look: 'out-of-character',
      label: 'Celso · fora do personagem',
    });
  });

  it('an erased account reads "Jogador removido"', () => {
    const msg = line('1', { speakerKind: Kind.OUT_OF_CHARACTER, authorRemoved: true });
    expect(viewLine(msg, master, directory).label).toBe('Jogador removido');
  });

  it("a player reads the master's chosen-readers line as a whisper", () => {
    const msg = line('1', { speakerKind: Kind.NARRATOR, forEveryone: false });
    expect(viewLine(msg, player, directory)).toMatchObject({
      look: 'restricted',
      label: 'Sussurro do mestre · só você lê',
      audience: null,
    });
    const npc = line('2', { speakerKind: Kind.NPC, speakerName: 'Bartolo', forEveryone: false });
    expect(viewLine(npc, player, directory).label).toBe('Sussurro do mestre · só você lê');
  });

  it('the master reads who a narration is for, from the reader ids', () => {
    const msg = line('1', {
      speakerKind: Kind.NARRATOR,
      forEveryone: false,
      authorUserId: 'u-mestre',
      readerUserIds: ['u-pen', 'u-bri'],
    });
    expect(viewLine(msg, master, directory)).toMatchObject({
      look: 'restricted',
      label: 'Narração · só para Pensantus e Brisa',
      audience: 'Só para Pensantus e Brisa',
    });
  });

  it('the master reads who an NPC line is for, with the NPC kept in the label', () => {
    const msg = line('1', {
      speakerKind: Kind.NPC,
      speakerName: 'Homem do canto',
      forEveryone: false,
      authorUserId: 'u-mestre',
      readerUserIds: ['u-pen'],
    });
    expect(viewLine(msg, master, directory)).toMatchObject({
      look: 'restricted',
      label: 'Homem do canto · você · só para Pensantus',
      audience: 'Só para Pensantus',
    });
  });

  it('a reader the directory does not know does not break the line', () => {
    const msg = line('1', {
      speakerKind: Kind.NARRATOR,
      forEveryone: false,
      readerUserIds: ['u-pen', 'u-x'],
    });
    expect(viewLine(msg, master, directory).audience).toBe('Só para Pensantus e um jogador');
  });

  it("a player's line for the master alone says so, to the author and to the master", () => {
    const msg = line('1', {
      speakerKind: Kind.CHARACTER,
      speakerName: 'Toren',
      forEveryone: false,
      authorUserId: 'u-tor',
    });
    expect(viewLine(msg, player, directory)).toMatchObject({
      look: 'restricted',
      label: 'Toren · você · só o mestre lê',
    });
    expect(viewLine(msg, master, directory).label).toBe('Toren · só o mestre lê');
  });

  it('carries what the line needs to be heard and shown', () => {
    const msg = line('1', {
      canHear: true,
      audioUrl: '/session-messages/1/audio',
      image: { id: 'i', name: 'Taverna', url: '/images/i', thumbnailUrl: '/images/i/thumb' },
    });
    const v = viewLine(msg, player, directory);
    expect(v).toMatchObject({ canHear: true, audioUrl: '/session-messages/1/audio' });
    expect(v.image?.url).toBe('/images/i');
  });
});

describe('the readers texts', () => {
  it('joins names in Portuguese', () => {
    expect(joinNames([])).toBe('');
    expect(joinNames(['A'])).toBe('A');
    expect(joinNames(['A', 'B'])).toBe('A e B');
    expect(joinNames(['A', 'B', 'C'])).toBe('A, B e C');
  });

  it("the master's 'Para' button says 'Todos' or the names", () => {
    expect(readersSummary([], directory)).toBe('Todos');
    expect(readersSummary(['u-pen', 'u-bri', 'u-tor'], directory)).toBe('Pensantus, Brisa e Toren');
  });

  it("the chooser's live line follows the choice", () => {
    expect(chooserLine(['Pensantus', 'Brisa'], false)).toBe(
      'Pensantus e Brisa leem esta mensagem.',
    );
    expect(chooserLine(['Pensantus'], false)).toBe('Pensantus lê esta mensagem.');
    expect(chooserLine([], false)).toBe('Ninguém foi escolhido. Marque quem lê esta mensagem.');
    expect(chooserLine([], true)).toBe('Toda a mesa lê esta mensagem.');
  });
});

describe('buildSendRequest', () => {
  const key = 'key-1';

  it('trims the text and refuses an empty one or one over the limit', () => {
    const empty = buildSendRequest(
      { role: 'player', text: '  \n ', as: 'out-of-character', characterId: '', onlyMaster: false },
      'c1',
      key,
    );
    expect(empty).toEqual({ ok: false, error: 'Escreva uma mensagem.' });
    const long = buildSendRequest(
      {
        role: 'player',
        text: 'x'.repeat(MAX_TEXT + 1),
        as: 'out-of-character',
        characterId: '',
        onlyMaster: false,
      },
      'c1',
      key,
    );
    expect(long.ok).toBe(false);
    const edge = buildSendRequest(
      {
        role: 'player',
        text: ` ${'x'.repeat(MAX_TEXT)} `,
        as: 'out-of-character',
        characterId: '',
        onlyMaster: false,
      },
      'c1',
      key,
    );
    expect(edge.ok).toBe(true);
  });

  it('a player speaks as the own character, with its id', () => {
    const built = buildSendRequest(
      { role: 'player', text: ' Oi ', as: 'character', characterId: 'ch1', onlyMaster: false },
      'c1',
      key,
    );
    expect(built).toMatchObject({
      ok: true,
      request: {
        campaignId: 'c1',
        speakerKind: Kind.CHARACTER,
        speakerId: 'ch1',
        text: 'Oi',
        onlyMaster: false,
        readerUserIds: [],
        imageId: '',
        idempotencyKey: key,
      },
    });
  });

  it('a player out of character names no speaker, and "só o mestre" is a flag', () => {
    const built = buildSendRequest(
      { role: 'player', text: 'Oi', as: 'out-of-character', characterId: 'ch1', onlyMaster: true },
      'c1',
      key,
    );
    expect(built).toMatchObject({
      ok: true,
      request: { speakerKind: Kind.OUT_OF_CHARACTER, speakerId: '', onlyMaster: true },
    });
  });

  it('a player without a living character cannot speak as one', () => {
    const built = buildSendRequest(
      { role: 'player', text: 'Oi', as: 'character', characterId: '', onlyMaster: false },
      'c1',
      key,
    );
    expect(built.ok).toBe(false);
  });

  it('the master narrates to everyone or to chosen readers, once each, with an image', () => {
    const built = buildSendRequest(
      {
        role: 'master',
        text: 'A porta range.',
        speaker: { kind: 'narrator' },
        readerUserIds: ['u-pen', 'u-pen', 'u-bri'],
        imageId: 'img',
      },
      'c1',
      key,
    );
    expect(built).toMatchObject({
      ok: true,
      request: {
        speakerKind: Kind.NARRATOR,
        speakerId: '',
        readerUserIds: ['u-pen', 'u-bri'],
        imageId: 'img',
        onlyMaster: false,
      },
    });
  });

  it('the master speaks as an NPC, which needs its id', () => {
    const ok = buildSendRequest(
      {
        role: 'master',
        text: 'Quartos, só amanhã.',
        speaker: { kind: 'npc', npcId: 'n1' },
        readerUserIds: [],
      },
      'c1',
      key,
    );
    expect(ok).toMatchObject({ ok: true, request: { speakerKind: Kind.NPC, speakerId: 'n1' } });
    const bad = buildSendRequest(
      { role: 'master', text: 'Oi', speaker: { kind: 'npc', npcId: '' }, readerUserIds: [] },
      'c1',
      key,
    );
    expect(bad.ok).toBe(false);
  });
});
