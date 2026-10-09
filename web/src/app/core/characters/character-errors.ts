import { Code, ConnectError } from '@connectrpc/connect';

import {
  AbilityScoresRefusalReason,
  AbilityScoresRefusalSchema,
  CharacterBlockedReason as GenCharacterBlockedReason,
  CharacterBlockedSchema,
  InvalidFieldSchema,
  LevelUpRefusalReason,
  LevelUpRefusalSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { describeConnectError } from '../connect/connect-errors';
import { CharacterBlockedReason } from './characters.types';

/**
 * The content key a player's save was refused for because the master switched it off in "Opções para os jogadores" (RN-23):
 * `failed_precondition` with `CharacterBlocked` `SWITCHED_OFF_CONTENT` and the key ("race:tiefling", "class:wizard"). `null` for
 * any other error. The editor shows it on the field that holds the key, by the typed reason and never by the message.
 */
export function switchedOffKey(err: unknown): string | null {
  const e = ConnectError.from(err, Code.Unavailable);
  if (e.code !== Code.FailedPrecondition) {
    return null;
  }
  const [detail] = e.findDetails(CharacterBlockedSchema);
  return detail?.reason === GenCharacterBlockedReason.SWITCHED_OFF_CONTENT
    ? detail.contentKey || ''
    : null;
}

/** The living character that stops a revival (RN-03): what the card "já tem outro personagem vivo" names and links to. */
export interface LivingRefusal {
  readonly characterId: string;
  readonly name: string;
}

/**
 * The living character of the player that refused a revival (`failed_precondition` with `CharacterBlocked`
 * `LIVING_CHARACTER_EXISTS`, `ReviveCharacter` and `ConfirmRevivifyTime`), or `null` for any other error.
 */
export function livingRefusal(err: unknown): LivingRefusal | null {
  const e = ConnectError.from(err, Code.Unavailable);
  if (e.code !== Code.FailedPrecondition) {
    return null;
  }
  const [detail] = e.findDetails(CharacterBlockedSchema);
  return detail?.reason === GenCharacterBlockedReason.LIVING_CHARACTER_EXISTS
    ? { characterId: detail.characterId, name: detail.livingCharacterName }
    : null;
}

/**
 * Turns a `CharacterBlocked.reason` into the message the sheet and the
 * editor show as-is. Kept separate from `describeCharacterError` so both can
 * be unit-tested without a `ConnectError` in hand — this one takes the
 * already-decoded local reason, not a wire enum.
 */
const BLOCKED_MESSAGES: Partial<Record<CharacterBlockedReason, string>> = {
  sheet_locked:
    'A ficha está travada porque a campanha já começou a jogar. Só o mestre pode editá-la agora.',
  character_dead: 'Esse personagem está morto e a ficha não pode mais ser editada.',
  living_character_exists: 'Você já tem um personagem vivo nesta campanha.',
  story_locked:
    'O mestre ainda não liberou a edição da história. Peça para ele liberar em "Permitir editar a história".',
  not_pending:
    'Esse personagem já foi aprovado e faz parte da campanha: não dá mais para recusá-lo nem pedir ajustes.',
  no_changes_requested:
    'O mestre não tem um pedido de ajustes aberto nesse personagem. Atualize a página.',
  not_dead: 'Esse personagem não está morto. Atualize a página.',
  awaiting_approval: 'Esse personagem ainda espera a sua aprovação. Aprove ou recuse antes.',
  not_reserved: 'Esse personagem já tem dono. Devolva-o à reserva antes de gerar um link.',
  claim_link_used: 'Esse link já foi usado: um jogador assumiu o personagem.',
  not_claimed:
    'Esse personagem não veio de um link, ou já está na reserva: não dá para devolvê-lo.',
  character_in_combat:
    'Esse personagem está em um combate. Encerre o combate antes de devolvê-lo à reserva.',
  claim_own_link: 'Este link é para um jogador. Copie e envie para ele.',
  reserved: 'Esse personagem está reservado: ainda não tem jogador.',
};

export function characterBlockedMessage(
  reason: CharacterBlockedReason | undefined,
  content?: ContentRef,
): string {
  if (reason === 'archived_content' || reason === 'switched_off_content') {
    const what = reason === 'archived_content' ? 'arquivad' : 'desligad';
    const by =
      reason === 'archived_content'
        ? 'pelo mestre e não vale mais como escolha nova'
        : 'pelo mestre para os jogadores';
    return `${contentWords(content)} foi ${what}${content?.masculine ? 'o' : 'a'} ${by}. Escolha outra opção${content?.step ? `, no passo ${content.step}` : ''}.`;
  }
  return (reason && BLOCKED_MESSAGES[reason]) || 'Não foi possível concluir a ação agora.';
}

/** What a content key is, in words, for an error that names one: "A classe “Guardião do Vale”". */
export interface ContentRef {
  /** The Portuguese name from the catalog; empty when the catalog does not know the key. */
  readonly name: string;
  /** The kind of entry, from the key: "a classe", "o antecedente"... */
  readonly noun: string;
  readonly masculine: boolean;
  /** The step of the editor that holds the field. */
  readonly step: string;
}

const KEY_KINDS: Record<string, { noun: string; masculine: boolean; step: string }> = {
  class: { noun: 'classe', masculine: false, step: 'Básico' },
  subclass: { noun: 'subclasse', masculine: false, step: 'Básico' },
  race: { noun: 'raça', masculine: false, step: 'Básico' },
  subrace: { noun: 'sub-raça', masculine: false, step: 'Básico' },
  background: { noun: 'antecedente', masculine: true, step: 'Básico' },
  spell: { noun: 'magia', masculine: false, step: 'Magias' },
};

/** The words for a content key: its kind from the key, its name from `nameOf` (never the key itself). */
export function contentRef(key: string, nameOf: (key: string) => string | undefined): ContentRef {
  const kind = KEY_KINDS[key.split(':')[0]] ?? { noun: 'opção', masculine: false, step: '' };
  return { name: nameOf(key) ?? '', ...kind };
}

function contentWords(content: ContentRef | undefined): string {
  if (!content) {
    return 'Uma das opções';
  }
  const article = content.masculine ? 'O' : 'A';
  return content.name
    ? `${article} ${content.noun} “${content.name}”`
    : `${article} ${content.noun} escolhid${content.masculine ? 'o' : 'a'}`;
}

/** The sheet field an `invalid_argument` points at ("full.classes[1].class_key"), from its typed detail; `null` when it has none. */
export function invalidFieldPath(err: unknown): string | null {
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code !== Code.InvalidArgument) {
    return null;
  }
  return connectErr.findDetails(InvalidFieldSchema)[0]?.field ?? null;
}

const BLOCKED_FROM_GEN: Partial<Record<GenCharacterBlockedReason, CharacterBlockedReason>> = {
  [GenCharacterBlockedReason.SHEET_LOCKED]: 'sheet_locked',
  [GenCharacterBlockedReason.CHARACTER_DEAD]: 'character_dead',
  [GenCharacterBlockedReason.LIVING_CHARACTER_EXISTS]: 'living_character_exists',
  [GenCharacterBlockedReason.STORY_LOCKED]: 'story_locked',
  [GenCharacterBlockedReason.NOT_PENDING]: 'not_pending',
  [GenCharacterBlockedReason.AWAITING_APPROVAL]: 'awaiting_approval',
  [GenCharacterBlockedReason.NO_CHANGES_REQUESTED]: 'no_changes_requested',
  [GenCharacterBlockedReason.NOT_DEAD]: 'not_dead',
  [GenCharacterBlockedReason.ARCHIVED_CONTENT]: 'archived_content',
  [GenCharacterBlockedReason.SWITCHED_OFF_CONTENT]: 'switched_off_content',
  [GenCharacterBlockedReason.NOT_RESERVED]: 'not_reserved',
  [GenCharacterBlockedReason.CLAIM_LINK_USED]: 'claim_link_used',
  [GenCharacterBlockedReason.NOT_CLAIMED]: 'not_claimed',
  [GenCharacterBlockedReason.CHARACTER_IN_COMBAT]: 'character_in_combat',
  [GenCharacterBlockedReason.CLAIM_OWN_LINK]: 'claim_own_link',
  [GenCharacterBlockedReason.RESERVED]: 'reserved',
};

/** Maps the wire `CharacterBlockedReason` enum (characters.proto) onto the
 * local, UI-facing union `characterBlockedMessage` reads. `UNSPECIFIED` and
 * any future value this app does not know about yet fall through to
 * `undefined`, which `characterBlockedMessage` already turns into a safe
 * generic message instead of throwing. */
function mapBlockedReason(
  reason: GenCharacterBlockedReason | undefined,
): CharacterBlockedReason | undefined {
  return reason === undefined ? undefined : BLOCKED_FROM_GEN[reason];
}

/** The typed reason of an `AbilityScoresRefusal` an error carries, or `null` for any other error. */
export function abilityRefusalReason(err: unknown): AbilityScoresRefusalReason | null {
  const connectErr = ConnectError.from(err, Code.Unavailable);
  if (connectErr.code !== Code.FailedPrecondition) {
    return null;
  }
  return connectErr.findDetails(AbilityScoresRefusalSchema)[0]?.reason ?? null;
}

/** What each `AbilityScoresRefusal` says (RN-24): by the typed reason, never by the server's message. */
export function abilityRefusalMessage(reason: AbilityScoresRefusalReason): string {
  switch (reason) {
    case AbilityScoresRefusalReason.METHOD_NOT_ALLOWED:
      return 'O mestre não liberou esse jeito de fazer as habilidades nesta mesa. Escolha outro.';
    case AbilityScoresRefusalReason.NOT_STANDARD_ARRAY:
      return 'Os valores não são o conjunto padrão: cada valor do conjunto vai em uma habilidade, uma vez só.';
    case AbilityScoresRefusalReason.BAD_POINT_BUY:
      return 'A compra por pontos passou do total de pontos, ou tem um valor fora do limite. Confira os valores.';
    case AbilityScoresRefusalReason.NO_ROLLS_STORED:
      return 'Role as habilidades antes de criar o personagem.';
    case AbilityScoresRefusalReason.NOT_THE_ROLLS:
      return 'Os valores não são os seis resultados rolados. Use cada resultado uma vez.';
    case AbilityScoresRefusalReason.TYPED_OUT_OF_RANGE:
      return 'Um valor digitado está fora do limite da mesa. Confira os seis valores.';
    case AbilityScoresRefusalReason.DICE_FORCED_IN_APP:
      return 'Nesta campanha todos rolam no app: peça a rolagem ao servidor, sem digitar dados.';
    case AbilityScoresRefusalReason.DICE_FORCED_PHYSICAL:
      return 'Nesta campanha todos usam os próprios dados: digite os dados que você tirou.';
    case AbilityScoresRefusalReason.ROLLS_ALREADY_STORED:
      return 'Os dados já foram guardados e não mudam. Use os resultados que aparecem.';
    case AbilityScoresRefusalReason.EXTRA_BONUSES:
      return 'Os bônus manuais passam do que a raça e os incrementos no valor de habilidade deixam pôr.';
    default:
      return 'As habilidades não seguem as regras da mesa. Confira o passo "Habilidades".';
  }
}

/**
 * Maps any `CharacterService` error to a message a form can show as-is.
 *
 * For `failed_precondition`, this reads the `CharacterBlocked` detail off
 * the error itself (`findDetails(CharacterBlockedSchema)`) — the caller
 * never needs to guess or pass a reason in.
 */
export function describeCharacterError(
  err: unknown,
  nameOf?: (key: string) => string | undefined,
): string {
  const connectErr = ConnectError.from(err, Code.Unavailable);

  if (connectErr.code === Code.FailedPrecondition) {
    const [refusal] = connectErr.findDetails(AbilityScoresRefusalSchema);
    if (refusal) {
      return abilityRefusalMessage(refusal.reason);
    }
    // A new sheet whose hit points above level 1 are not what the table's rule allows (RN-24).
    const [hp] = connectErr.findDetails(LevelUpRefusalSchema);
    if (hp?.reason === LevelUpRefusalReason.HIT_POINTS_RULE) {
      return 'A mesa decidiu como se ganham os pontos de vida dos níveis acima do 1º. Use o jeito que ela deixa, no passo "Habilidades".';
    }
    const [detail] = connectErr.findDetails(CharacterBlockedSchema);
    const content = detail?.contentKey
      ? contentRef(detail.contentKey, nameOf ?? (() => undefined))
      : undefined;
    return characterBlockedMessage(mapBlockedReason(detail?.reason), content);
  }
  if (connectErr.code === Code.Aborted) {
    // AIP-154-style stale revision: someone else (the player, the master,
    // or the server locking the sheet) saved first.
    return 'A ficha mudou enquanto você editava. Recarregue a página e tente de novo.';
  }

  return describeConnectError(connectErr, {
    [Code.PermissionDenied]: 'Você não tem permissão para fazer isso.',
    [Code.NotFound]: 'Personagem não encontrado.',
    [Code.ResourceExhausted]:
      'A campanha chegou ao limite de 1.000 personagens e NPCs. Apague um para criar outro.',
    [Code.InvalidArgument]: invalidArgumentMessage(connectErr),
  });
}

/** "Classe 2: ..." when the server points at a class block; the generic line otherwise. */
function invalidArgumentMessage(err: ConnectError): string {
  const field = err.findDetails(InvalidFieldSchema)[0]?.field ?? '';
  if (field === 'reason') {
    return 'O motivo precisa ter de 1 a 500 caracteres.';
  }
  const block = /^full\.classes\[(\d+)\]/.exec(field);
  if (block) {
    return `Classe ${Number(block[1]) + 1}: essa classe se repete ou não existe. Cada classe entra uma vez só; escolha outra no passo Básico.`;
  }
  return 'Confira os campos da ficha.';
}
