import { ImageAspectRatio, type ImageStyle } from '../../../gen/meurpg/maps/v1/imagegen_pb';
import type { MapRequest, SceneRequest } from './imagegen-client';
import { DEFAULT_MAX_PROMPT, type KindKey, mapKindOf } from './imagegen-copy';

/** The dialog's fields, as the master fills them. The ids are the ones the server sent (gallery images, NPCs of the players' view). */
export interface ImageForm {
  readonly kind: KindKey;
  readonly prompt: string;
  /** The gallery image's name: the players read it when the master shows the image. Empty lets the server name it. */
  readonly name: string;
  readonly style: ImageStyle;
  /** 0 asks for the server's default for the way (16:9). The textured map has no ratio: the server picks one from the grid. */
  readonly ratio: ImageAspectRatio;
  /** Gallery images chosen as references (objects and places). */
  readonly objectImageIds: readonly string[];
  /** The NPCs marked "Quem aparece na imagem". */
  readonly npcIds: readonly string[];
}

/** An NPC of the players' view, as `GetMapImageReference` lists it. */
export interface PickableNpc {
  readonly characterId: string;
  readonly name: string;
  readonly portraitImageId: string;
}

/** The request the form makes, from a map (`GenerateMapImage`) or from nothing (`GenerateSceneImage`). */
export type Built = { readonly via: 'map'; readonly request: MapRequest } | { readonly via: 'scene'; readonly request: SceneRequest };

/**
 * The form → request mapping (RN-28, E10-07). The way of a map takes the NPCs marked and only theirs: the server turns each portrait into a
 * character reference, so the app sends the NPC ids and never their portraits as `character_image_ids`. The textured map takes no
 * creature at all (`npc_character_ids` and `character_image_ids` must be empty) and no ratio, so both are left out; a scene without a map
 * sends the text, the style, the ratio and the references only.
 */
export function buildRequest(form: ImageForm, mapId: string | null, key: string): Built {
  const prompt = form.prompt.trim();
  if (mapId === null) {
    return { via: 'scene', request: { idempotencyKey: key, prompt, name: form.name.trim(), style: form.style, aspectRatio: form.ratio, objectImageIds: [...form.objectImageIds], characterImageIds: [] } };
  }
  const texture = form.kind === 'texture';
  return {
    via: 'map',
    request: {
      mapId,
      kind: mapKindOf(form.kind),
      idempotencyKey: key,
      prompt,
      name: form.name.trim(),
      style: form.style,
      aspectRatio: texture ? ImageAspectRatio.IMAGE_ASPECT_RATIO_UNSPECIFIED : form.ratio,
      objectImageIds: [...form.objectImageIds],
      characterImageIds: [],
      npcCharacterIds: texture ? [] : [...form.npcIds],
    },
  };
}

/** What "Gerar imagem" is waiting for, or `null` when the form can be sent: the text (the artboard keeps the button dashed until there is some). */
export function formProblem(form: ImageForm, maxPrompt = DEFAULT_MAX_PROMPT): string | null {
  const length = [...form.prompt.trim()].length;
  if (length === 0) {
    return 'Escreva o que a imagem mostra para gerar.';
  }
  if (length > maxPrompt) {
    return `O texto vai até ${maxPrompt} caracteres.`;
  }
  return null;
}

/** How many NPC portraits go as references: the marked NPCs that have one, each portrait once (the server counts them against the 4). */
export function npcPortraits(form: ImageForm, npcs: readonly PickableNpc[]): string[] {
  if (form.kind === 'texture') {
    return [];
  }
  const ids = new Set<string>();
  for (const npc of npcs) {
    if (form.npcIds.includes(npc.characterId) && npc.portraitImageId !== '') {
      ids.add(npc.portraitImageId);
    }
  }
  return [...ids];
}
