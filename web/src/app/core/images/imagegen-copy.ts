import { ImageAspectRatio, ImageGenerationKind, type ImageGenerationStatus, ImageStyle } from '../../../gen/meurpg/maps/v1/imagegen_pb';
import { tight } from '../format/text';

/** The three ways to ask for a picture (E10-07), by the name the form uses. */
export type KindKey = 'scene' | 'isometric' | 'texture';

/** The kind's name on a computer and on a phone (the sheet's segments say "Cena", "Isométrica" and "Textura"). */
export const KIND_LABEL: Readonly<Record<KindKey, { readonly long: string; readonly short: string }>> = {
  scene: { long: 'Arte da cena', short: 'Cena' },
  isometric: { long: 'Vista isométrica', short: 'Isométrica' },
  texture: { long: 'O mapa com textura', short: 'Textura' },
};

/** What each way makes, said under the choice. */
export const KIND_ABOUT: Readonly<Record<KindKey, string>> = {
  scene: 'Uma pintura do lugar. Não casa com a grade.',
  isometric: 'O mapa visto de cima e de lado, com os inimigos que você marcar. Não casa com a grade.',
  texture: 'O próprio mapa visto de cima, ajustado à grade.',
};

/** The request's `kind` for a way, from a map (the scene art of a map starts from what the players see). */
export function mapKindOf(key: KindKey): ImageGenerationKind {
  switch (key) {
    case 'isometric':
      return ImageGenerationKind.ISOMETRIC;
    case 'texture':
      return ImageGenerationKind.TEXTURED_MAP;
    default:
      return ImageGenerationKind.MAP_SCENE;
  }
}

/** Where the dialog was opened from: a map (with a grid or without), a scene of RP, or the gallery. */
export interface GenerateOrigin {
  readonly kind: 'map' | 'scene' | 'gallery';
  /** A map: its id and name. A scene: the point's name, which starts the text. */
  readonly mapId?: string;
  readonly name?: string;
  /** A map without a grid cannot be drawn from. */
  readonly hasGrid?: boolean;
  /** The map or the point is revealed to the players: only then its name may name the picture (RN-10). */
  readonly revealed?: boolean;
}

export interface KindChoice {
  readonly key: KindKey;
  readonly available: boolean;
  /** Why it is off, said under the group. */
  readonly reason: string;
}

/** The reason the maps' two ways are off without a map. */
export const NEEDS_A_MAP = 'As outras duas precisam de um mapa com grade. Abra o diálogo a partir de um mapa para usá‑las.';
export const NEEDS_A_GRID = 'Este mapa não tem grade, e as outras duas precisam dela. Defina a grade do mapa para usá-las.';
export const NO_PLAYERS_VIEW = 'Nenhum personagem de jogador vê este mapa agora, então não há o que a imagem mostrar.';

/** The ways the dialog offers, with the dashed ones' reasons (E10-07 2 and 3). `textureTooLarge` comes from `GetMapImageReference`. */
export function kindChoices(origin: GenerateOrigin, textureTooLarge: boolean): KindChoice[] {
  if (origin.kind !== 'map' || origin.hasGrid === false) {
    const reason = origin.kind === 'map' ? NEEDS_A_GRID : NEEDS_A_MAP;
    return [
      { key: 'scene', available: true, reason: '' },
      { key: 'isometric', available: false, reason },
      { key: 'texture', available: false, reason },
    ];
  }
  return [
    { key: 'scene', available: true, reason: '' },
    { key: 'isometric', available: true, reason: '' },
    {
      key: 'texture',
      available: !textureTooLarge,
      reason: textureTooLarge ? 'A imagem deste mapa tem mais de 16 megapixels (4.000 × 4.000 px). Troque por uma menor para usar o mapa com textura.' : '',
    },
  ];
}

/** The styles, in the order of the artboard ("Sem estilo" last). */
export const STYLES: readonly { readonly value: ImageStyle; readonly label: string }[] = [
  { value: ImageStyle.OIL_PAINTING, label: 'Pintura a óleo' },
  { value: ImageStyle.WATERCOLOR, label: 'Aquarela' },
  { value: ImageStyle.INK, label: 'Nanquim' },
  { value: ImageStyle.DIGITAL_ART, label: 'Arte digital' },
  { value: ImageStyle.BOOK_ILLUSTRATION, label: 'Ilustração de livro' },
  { value: ImageStyle.UNSPECIFIED, label: 'Sem estilo' },
];

export function styleLabel(style: ImageStyle): string {
  return STYLES.find((s) => s.value === style)?.label ?? 'Sem estilo';
}

/** The ratios the model returns; the server picks 16:9 when none is sent. */
export const RATIOS: readonly { readonly value: ImageAspectRatio; readonly label: string }[] = [
  { value: ImageAspectRatio.IMAGE_ASPECT_RATIO_UNSPECIFIED, label: 'Padrão (16:9)' },
  { value: ImageAspectRatio.IMAGE_ASPECT_RATIO_1_1, label: '1:1 (quadrada)' },
  { value: ImageAspectRatio.IMAGE_ASPECT_RATIO_3_2, label: '3:2' },
  { value: ImageAspectRatio.IMAGE_ASPECT_RATIO_2_3, label: '2:3 (em pé)' },
  { value: ImageAspectRatio.IMAGE_ASPECT_RATIO_3_4, label: '3:4' },
  { value: ImageAspectRatio.IMAGE_ASPECT_RATIO_4_3, label: '4:3' },
  { value: ImageAspectRatio.IMAGE_ASPECT_RATIO_4_5, label: '4:5' },
  { value: ImageAspectRatio.IMAGE_ASPECT_RATIO_5_4, label: '5:4' },
  { value: ImageAspectRatio.IMAGE_ASPECT_RATIO_9_16, label: '9:16 (em pé)' },
  { value: ImageAspectRatio.IMAGE_ASPECT_RATIO_16_9, label: '16:9' },
  { value: ImageAspectRatio.IMAGE_ASPECT_RATIO_21_9, label: '21:9 (panorâmica)' },
];

/** The text limits and counts the form shows: the server's own (`ImageGenerationStatus`), these only when it did not answer. */
export const DEFAULT_MAX_PROMPT = 500;
export const DEFAULT_MAX_OBJECTS = 10;
export const DEFAULT_MAX_CHARACTERS = 4;

/** The sentence under the text field: what goes to Google, said where one writes (E10-07, B22). */
export const PROMPT_PRIVACY = 'O texto que você escrever e as imagens que escolher como referência vão para o Google (API do Gemini) para gerar a imagem. Não escreva nomes de pessoas.';
export const STYLE_PRIVACY = 'O estilo vai ao Google como uma palavra, junto com o texto.';
export const RATIO_PRIVACY = 'A proporção vai ao Google junto com o texto.';
export const REFERENCES_PRIVACY = 'As imagens escolhidas vão ao Google como referência. Não use foto de pessoa.';
export const EDIT_PRIVACY = 'O ajuste que você escrever vai ao Google, junto com a imagem anterior e o seu pedido de antes. Não escreva nomes de pessoas.';
export const SYNTHID = 'As imagens levam uma marca d’água invisível (SynthID), que mostra que foram feitas por IA.';

/** What goes with a request made from a map, by way (the box "O que vai junto"). `rooms` is how many rooms of a generated dungeon go in the text. */
export function whatGoesAlong(key: KindKey, rooms: number): { readonly lead: string; readonly body: string } {
  if (key === 'texture') {
    return {
      lead: 'O desenho do mapa inteiro.',
      body:
        (rooms > 0 ? `O desenho do chão e das paredes e a lista das ${rooms} salas, com o nome e o tamanho de cada uma.` : 'O desenho do chão e das paredes.') +
        ' Sem as portas e sem números. A porta secreta vai como parede.',
    };
  }
  return {
    lead: 'A imagem parte do que os jogadores veem agora.',
    body: 'Só os quadrados que algum personagem vê e as criaturas que eles veem. Sem as portas e sem números; uma porta secreta que ninguém achou é parede.',
  };
}

const MONTHS = ['janeiro', 'fevereiro', 'março', 'abril', 'maio', 'junho', 'julho', 'agosto', 'setembro', 'outubro', 'novembro', 'dezembro'];

/** "outubro", from the server's month ("2026-10"). */
export function monthName(month: string): string {
  return MONTHS[Number(month.slice(5, 7)) - 1] ?? 'este mês';
}

/** "Restam 17 de 20 imagens em outubro." */
export function remainingText(status: Pick<ImageGenerationStatus, 'remaining' | 'monthlyLimit' | 'month'>): string {
  const n = status.remaining;
  return tight(`${n === 1 ? 'Resta' : 'Restam'} ${n} de ${status.monthlyLimit} imagens em ${monthName(status.month)}.`);
}

/** "1º de novembro": the day the count starts over, in Brazil's time. */
export function resetText(status: Pick<ImageGenerationStatus, 'resetsAt'>): string {
  const at = status.resetsAt ? new Date(Number(status.resetsAt.seconds) * 1000) : null;
  if (!at) {
    return 'no mês que vem';
  }
  const parts = new Intl.DateTimeFormat('pt-BR', { day: 'numeric', month: 'long', timeZone: 'America/Sao_Paulo' }).formatToParts(at);
  const day = parts.find((p) => p.type === 'day')?.value ?? '1';
  const month = parts.find((p) => p.type === 'month')?.value ?? 'mês';
  return `${day === '1' ? '1º' : day} de ${month}`;
}

/** What a request is called in the line over its text: "Arte da cena · Pintura a óleo · 2 NPCs". */
export function requestLine(key: KindKey | 'edit', style: ImageStyle, npcs: number): string {
  const parts = [key === 'edit' ? 'Ajuste' : KIND_LABEL[key].long];
  if (key !== 'edit') {
    parts.push(styleLabel(style));
    if (npcs > 0) {
      parts.push(npcs === 1 ? '1 NPC' : `${npcs} NPCs`);
    }
  }
  return parts.join(' · ');
}

/** The line under the picture (E10-07 5 and 6): how it was made. `grid` is the map's size in squares (the textured map's caption). */
export function resultCaption(
  how: { readonly kind: KindKey; readonly fromMap: boolean; readonly style: ImageStyle; readonly npcs: number; readonly grid?: string } | { readonly edit: string },
): string {
  if ('edit' in how) {
    return `Ajuste pedido: ${how.edit}`;
  }
  if (how.kind === 'texture') {
    return how.grid ? `O mapa visto de cima, com a grade por cima (${how.grid} quadrados)` : 'O mapa visto de cima, com a grade por cima';
  }
  const parts = [how.fromMap ? 'Gerada a partir do mapa' : 'Gerada a partir da cena', styleLabel(how.style)];
  if (how.npcs > 0) {
    parts.push(how.npcs === 1 ? '1 NPC' : `${how.npcs} NPCs`);
  }
  return parts.join(' · ');
}

const WAY_NAME: Record<KindKey, string> = { scene: 'arte da cena', isometric: 'vista isométrica', texture: 'mapa com textura' };
const WAY_TITLE: Record<KindKey, string> = { scene: 'Arte da cena', isometric: 'Vista isométrica', texture: 'Mapa com textura' };

/** The day, as the server writes it in a default name ("06/10", Brasília). */
function dayOf(now: Date): string {
  return new Intl.DateTimeFormat('pt-BR', { day: '2-digit', month: '2-digit', timeZone: 'America/Sao_Paulo' }).format(now);
}

/**
 * The default name of a picture, the same the server gives one without a name. The players read the name of a picture
 * they are shown (RN-10), so it never takes what the master wrote in the request, nor the name of a map or point they
 * don't see: a revealed map's name and the way ("Masmorra de Mirathel · vista isométrica"), a revealed scene's name,
 * otherwise the way and the day ("Arte da cena · 06/10").
 */
export function defaultImageName(origin: GenerateOrigin, kind: KindKey, now: Date = new Date()): string {
  // A map without a grid is made as a scene art from a text.
  const drawn = origin.kind === 'map' && origin.hasGrid !== false;
  const way: KindKey = drawn ? kind : 'scene';
  if (origin.revealed && origin.name) {
    return drawn ? `${origin.name} · ${WAY_NAME[way]}` : origin.name;
  }
  return `${WAY_TITLE[way]} · ${dayOf(now)}`;
}

/** The longest name a gallery image has. */
export const MAX_IMAGE_NAME = 80;
