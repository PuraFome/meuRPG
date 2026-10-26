import type { SceneAudienceView, ScenePlayer } from '../../../gen/meurpg/play/v1/scene_pb';
import type { ImageAudienceVm } from './live-session.types';
import { joinNames } from './stage-view';

/** More names than this are counted ("6 de 8 jogadores") instead of listed. */
const NAMED_LIMIT = 3;

/** One row of the master's chooser of players: the account, the character's name and "na cena" / "fora da cena". */
export interface ChoosablePlayer {
  readonly userId: string;
  readonly name: string;
  readonly sub: string;
}

/** The players of the table, those in the scene first (C6-11): who the master can choose among. */
export function choosablePlayers(audience: SceneAudienceView | null): ChoosablePlayer[] {
  if (!audience) {
    return [];
  }
  return [
    ...audience.inside.map((p) => ({ userId: p.userId, name: p.characterName, sub: 'na cena' })),
    ...audience.outside.map((p) => ({
      userId: p.userId,
      name: p.characterName,
      sub: 'fora da cena',
    })),
  ];
}

/** Every player of the table, in or out. */
function everyPlayer(audience: SceneAudienceView): ScenePlayer[] {
  return [...audience.inside, ...audience.outside];
}

/** "Aberta desde 20:20 para …": the names as chips for up to three players, a count after that. */
export function insideSummary(audience: SceneAudienceView): {
  readonly chips: readonly string[];
  readonly count: string;
} {
  const names = audience.inside.map((p) => p.characterName);
  if (names.length <= NAMED_LIMIT) {
    return { chips: names, count: '' };
  }
  return { chips: names, count: `${names.length} de ${everyPlayer(audience).length} jogadores` };
}

/** The line under "Aberta desde …": who is out ("Toren não está nesta cena.", "Fora da cena: Sálvia e Jabuti."), or `''`. */
export function outsideLine(audience: SceneAudienceView): string {
  const names = audience.outside.map((p) => p.characterName);
  if (names.length === 0) {
    return '';
  }
  return names.length === 1
    ? `${names[0]} não está nesta cena.`
    : `Fora da cena: ${joinNames(names)}.`;
}

/** The chooser's live line for "Quem está na cena?". */
export function sceneChooserLine(names: readonly string[], everyone: boolean): string {
  if (everyone) {
    return 'Toda a mesa está na cena.';
  }
  if (names.length === 0) {
    return 'Ninguém foi escolhido. Marque quem está na cena.';
  }
  return `${joinNames(names)} ${names.length === 1 ? 'está' : 'estão'} na cena.`;
}

/** The chooser's live line for "Quem vê “<imagem>”?". */
export function imageChooserLine(names: readonly string[], everyone: boolean): string {
  if (everyone) {
    return 'Toda a mesa vê a imagem.';
  }
  if (names.length === 0) {
    return 'Ninguém foi escolhido. Marque quem vê a imagem.';
  }
  return `${joinNames(names)} ${names.length === 1 ? 'vê' : 'veem'} a imagem.`;
}

function seeing(names: readonly string[], total: number): string {
  if (names.length > NAMED_LIMIT) {
    return `${names.length} de ${total} jogadores veem`;
  }
  return `${joinNames(names)} ${names.length === 1 ? 'vê' : 'veem'}`;
}

/**
 * The line under the scene's images, who sees the one on display (C6-11): "Pensantus e Brisa veem",
 * "Só Brisa vê", "Toda a mesa vê" (when the scene is open to some players only) or "Os jogadores veem".
 */
export function shownAudienceLine(
  image: ImageAudienceVm | undefined,
  audience: SceneAudienceView | null,
): string {
  const everyone = !audience || audience.everyone;
  const kind = image?.kind ?? 'whole';
  if (kind === 'whole') {
    return everyone ? 'Os jogadores veem' : 'Toda a mesa vê';
  }
  if (!audience) {
    return 'Os jogadores veem';
  }
  const total = everyPlayer(audience).length;
  if (kind === 'scene') {
    return seeing(
      audience.inside.map((p) => p.characterName),
      total,
    );
  }
  const chosen = new Set(image?.userIds);
  const names = everyPlayer(audience)
    .filter((p) => chosen.has(p.userId))
    .map((p) => p.characterName);
  return names.length === 1 ? `Só ${names[0]} vê` : seeing(names, total);
}
