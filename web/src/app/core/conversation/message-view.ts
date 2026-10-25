import { create } from '@bufbuild/protobuf';

import {
  type SendSessionMessageRequest,
  SendSessionMessageRequestSchema,
  type SessionMessage,
  type SessionMessageImage,
  SessionMessageSpeakerKind as Kind,
} from '../../../gen/meurpg/play/v1/session_messages_pb';

/** The server refuses a line over this many characters (after trimming). */
export const MAX_TEXT = 2000;

/** The names of the table's players by account, as the page knows them (the
 * character's name, else the player's). The line only carries account ids. */
export type PlayerDirectory = ReadonlyMap<string, string>;

/** Who is reading. */
export interface Viewer {
  readonly userId: string;
  readonly isMaster: boolean;
}

/** The five looks of a line: narration on the page ground, a spoken line in a
 * bubble (an NPC, or another player's character), the reader's own line,
 * an out-of-character remark, and a line for chosen readers only. */
export type LineLook = 'narration' | 'npc' | 'own' | 'out-of-character' | 'restricted';

/** Everything a template needs to draw one line, already worded. */
export interface LineView {
  readonly id: string;
  readonly look: LineLook;
  /** The words above the line, as the board shows them. */
  readonly label: string;
  /** "Só para Pensantus e Brisa", for the master's line to chosen readers; `null` otherwise. */
  readonly audience: string | null;
  readonly speakerName: string;
  /** The letter in the round portrait when there is no image. */
  readonly initial: string;
  readonly portraitUrl: string | null;
  readonly text: string;
  readonly image: SessionMessageImage | null;
  readonly canHear: boolean;
  /** The reader wrote this line (their character, or the master's own NPC): it is not announced back to them. */
  readonly mine: boolean;
  /** The master spoke as an NPC: their own line to the right keeps the NPC's round portrait beside the bubble. */
  readonly npc?: boolean;
  readonly audioUrl: string;
}

const UNKNOWN_READER = 'um jogador';

/** "A", "A e B", "A, B e C". */
export function joinNames(names: readonly string[]): string {
  if (names.length < 2) {
    return names.join('');
  }
  return `${names.slice(0, -1).join(', ')} e ${names[names.length - 1]}`;
}

function readerNames(ids: readonly string[], directory: PlayerDirectory): string[] {
  return ids.map((id) => directory.get(id) ?? UNKNOWN_READER);
}

/** The text of the master's "Para" button. */
export function readersSummary(ids: readonly string[], directory: PlayerDirectory): string {
  return ids.length === 0 ? 'Todos' : joinNames(readerNames(ids, directory));
}

/** The chooser's live line (role=status). */
export function chooserLine(names: readonly string[], everyone: boolean): string {
  if (everyone) {
    return 'Toda a mesa lê esta mensagem.';
  }
  if (names.length === 0) {
    return 'Ninguém foi escolhido. Marque quem lê esta mensagem.';
  }
  return `${joinNames(names)} ${names.length === 1 ? 'lê' : 'leem'} esta mensagem.`;
}

function byMaster(message: SessionMessage): boolean {
  return message.speakerKind === Kind.NARRATOR || message.speakerKind === Kind.NPC;
}

function baseLook(message: SessionMessage, viewer: Viewer): Exclude<LineLook, 'restricted'> {
  switch (message.speakerKind) {
    case Kind.NARRATOR:
      // The master reads their own narration to the right, like any line of theirs (C3-01); the table reads it on the page.
      return isMine(message, viewer) ? 'own' : 'narration';
    case Kind.OUT_OF_CHARACTER:
      return 'out-of-character';
    default:
      return isMine(message, viewer) ? 'own' : 'npc';
  }
}

function speaks(message: SessionMessage): boolean {
  return message.speakerKind === Kind.CHARACTER || message.speakerKind === Kind.NPC;
}

function isMine(message: SessionMessage, viewer: Viewer): boolean {
  return message.authorUserId !== '' && message.authorUserId === viewer.userId;
}

function baseLabel(
  message: SessionMessage,
  look: LineLook,
  viewer: Viewer,
  directory: PlayerDirectory,
): string {
  const name = message.speakerName;
  switch (look) {
    case 'narration':
      return 'Narração';
    case 'own':
      return `${name || 'Narração'} · você`;
    case 'npc':
      return isMine(message, viewer) ? `${name} · você` : name;
    case 'out-of-character':
      // Only an erased account has no author id left. The server also flags a present author who never chose a
      // display name as removed, so the flag alone is not trusted: the table's name for the account, else "Jogador".
      return message.authorRemoved && message.authorUserId === ''
        ? 'Jogador removido'
        : `${name || directory.get(message.authorUserId) || 'Jogador'} · fora do personagem`;
    default:
      return name;
  }
}

/** The label and the audience of a line that not everyone reads. */
function restrictedWords(
  message: SessionMessage,
  viewer: Viewer,
  directory: PlayerDirectory,
  look: Exclude<LineLook, 'restricted'>,
): { label: string; audience: string | null } {
  // The player is told it is a whisper, never who else reads it.
  if (byMaster(message) && !viewer.isMaster) {
    return { label: 'Sussurro do mestre · só você lê', audience: null };
  }
  const base = baseLabel(message, look, viewer, directory);
  if (!byMaster(message)) {
    return { label: `${base} · só o mestre lê`, audience: null };
  }
  const audience = `Só para ${joinNames(readerNames(message.readerUserIds, directory))}`;
  return { label: `${base} · ${audience.charAt(0).toLowerCase()}${audience.slice(1)}`, audience };
}

/** Classifies a line for its reader and words it. The server already chose
 * what this reader may see (RN-10): nothing is filtered or counted here. */
export function viewLine(
  message: SessionMessage,
  viewer: Viewer,
  directory: PlayerDirectory,
): LineView {
  const base = baseLook(message, viewer);
  const restricted = !message.forEveryone;
  const look: LineLook = restricted ? 'restricted' : base;
  // A line for chosen readers keeps the words it always had: the label names the speaker, then who reads.
  const wordsLook: Exclude<LineLook, 'restricted'> =
    message.speakerKind === Kind.NARRATOR
      ? 'narration'
      : message.speakerKind === Kind.NPC
        ? 'npc'
        : base;
  const words = restricted
    ? restrictedWords(message, viewer, directory, wordsLook)
    : { label: baseLabel(message, base, viewer, directory), audience: null };
  const portrait = message.speakerPortrait;
  return {
    id: message.id,
    look,
    ...words,
    speakerName: message.speakerName,
    initial: Array.from(message.speakerName.trim())[0]?.toUpperCase() ?? '',
    portraitUrl: portrait ? portrait.thumbnailUrl || portrait.url : null,
    text: message.text,
    image: message.image ?? null,
    canHear: message.canHear,
    mine: speaks(message) && isMine(message, viewer),
    npc: message.speakerKind === Kind.NPC,
    audioUrl: message.audioUrl,
  };
}

/** What the composer holds, by role. */
export type Draft =
  | {
      readonly role: 'player';
      readonly text: string;
      readonly as: 'character' | 'out-of-character';
      /** The player's own living character, which a line "como <personagem>" names. */
      readonly characterId: string;
      /** "Para: só o mestre". */
      readonly onlyMaster: boolean;
    }
  | {
      readonly role: 'master';
      readonly text: string;
      readonly speaker:
        { readonly kind: 'narrator' } | { readonly kind: 'npc'; readonly npcId: string };
      /** Empty: the whole table. */
      readonly readerUserIds: readonly string[];
      readonly imageId?: string;
    };

export type BuiltRequest =
  | { readonly ok: true; readonly request: SendSessionMessageRequest }
  | { readonly ok: false; readonly error: string };

function textError(text: string): string | null {
  if (text.length === 0) {
    return 'Escreva uma mensagem.';
  }
  return text.length > MAX_TEXT ? 'A mensagem passa de 2.000 caracteres.' : null;
}

function refuse(error: string): BuiltRequest {
  return { ok: false, error };
}

/** The wire request for a draft, enforcing what the server enforces so a
 * mistake is told before the line is shown as sent: the text trimmed, 1 to
 * 2,000 characters; a player speaks as the own character or out of
 * character and never names readers; the master narrates or speaks as an NPC
 * and names readers, never "só o mestre". */
export function buildSendRequest(draft: Draft, campaignId: string, key: string): BuiltRequest {
  const text = draft.text.trim();
  const error = textError(text);
  if (error) {
    return refuse(error);
  }
  const common = { campaignId, text, idempotencyKey: key };
  if (draft.role === 'player') {
    if (draft.as === 'character' && !draft.characterId) {
      return refuse('Você não tem um personagem vivo para falar por ele.');
    }
    const asCharacter = draft.as === 'character';
    return {
      ok: true,
      request: create(SendSessionMessageRequestSchema, {
        ...common,
        speakerKind: asCharacter ? Kind.CHARACTER : Kind.OUT_OF_CHARACTER,
        speakerId: asCharacter ? draft.characterId : '',
        onlyMaster: draft.onlyMaster,
      }),
    };
  }
  if (draft.speaker.kind === 'npc' && !draft.speaker.npcId) {
    return refuse('Escolha o NPC que fala.');
  }
  return {
    ok: true,
    request: create(SendSessionMessageRequestSchema, {
      ...common,
      speakerKind: draft.speaker.kind === 'npc' ? Kind.NPC : Kind.NARRATOR,
      speakerId: draft.speaker.kind === 'npc' ? draft.speaker.npcId : '',
      readerUserIds: [...new Set(draft.readerUserIds)],
      imageId: draft.imageId ?? '',
    }),
  };
}
