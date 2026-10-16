import type { GalleryImage } from '../../../gen/meurpg/maps/v1/gallery_pb';
import {
  type ImageEdit,
  ImageGenerationBlockedReason,
  type ImageGenerationStatus,
} from '../../../gen/meurpg/maps/v1/imagegen_pb';
import {
  type GenerateOrigin,
  type KindKey,
  KIND_LABEL,
  requestLine,
  resultCaption,
} from '../../core/images/imagegen-copy';
import { type GenerateIssue, blockedText } from '../../core/images/imagegen-errors';
import { type ImageForm, formProblem } from '../../core/images/imagegen-form';
import type { ReferenceView } from './generate-form';
import type { ResultView } from './generate-result';
import type { RunFor, Stage } from './image-generate.types';

/** "Pedido: …" under an adjustment of the chain, empty without a text. */
const askedText = (prompt: string) => (prompt ? `Pedido: ${prompt}` : '');

/** The server's own reasons not to generate: the feature is off, or the month's slots are spent. */
function statusWhy(s: ImageGenerationStatus | null): string {
  if (s && !s.enabled) {
    return 'A geração de imagens não está ligada neste servidor.';
  }
  return s && s.remaining <= 0 ? blockedText(ImageGenerationBlockedReason.LIMIT_REACHED, s) : '';
}

export interface WhyInput {
  readonly status: ImageGenerationStatus | null;
  readonly form: ImageForm;
  /** The map the request is made from, or `null` (a scene, or a map without a grid). */
  readonly mapId: string | null;
  readonly referenceState: 'none' | 'loading' | 'ready' | 'error';
  /** The drawing for the way chosen. */
  readonly reference: ReferenceView | null;
}

/** Why "Gerar imagem" cannot be pressed yet, in words (the button is dashed and the line is above it); empty when it can. */
export function whyOf(i: WhyInput): string {
  const own = statusWhy(i.status);
  if (own !== '') {
    return own;
  }
  if (i.mapId !== null) {
    if (
      i.form.kind !== 'texture' &&
      i.referenceState === 'ready' &&
      i.reference?.playersSee === false
    ) {
      return blockedText(ImageGenerationBlockedReason.PLAYERS_SEE_NOTHING, null);
    }
    if (i.referenceState === 'loading') {
      return 'Montando o desenho que vai junto…';
    }
  }
  return formProblem(i.form, i.status?.maxPromptCharacters || undefined) ?? '';
}

/** Why "Pedir o ajuste" cannot be pressed yet. */
export function editWhyOf(status: ImageGenerationStatus | null, instruction: string): string {
  if (status && status.enabled && status.remaining <= 0) {
    return blockedText(ImageGenerationBlockedReason.LIMIT_REACHED, status);
  }
  return instruction.trim() === '' ? 'Escreva o ajuste que você quer pedir.' : '';
}

export interface SubtitleInput {
  readonly origin: GenerateOrigin;
  readonly stage: Stage;
  /** The request is still on its way to the server (not yet waiting for the picture). */
  readonly sending: boolean;
  readonly view: ResultView | null;
  readonly kindText: string;
}

/** The line under the title: where the picture comes from, or what the wait is. */
export function subtitleOf(i: SubtitleInput): string {
  const origin = i.origin;
  const place = origin.kind === 'scene' ? `Cena: ${origin.name ?? ''}` : (origin.name ?? '');
  if (i.stage === 'running') {
    return i.sending ? 'Antes de o pedido sair do servidor' : 'O servidor está gerando a imagem';
  }
  if (i.stage !== 'result') {
    return place;
  }
  // From the gallery there is no map to name: the way alone ("O mapa com textura").
  // The heading is the image's name: when it already holds the map's name the subtitle says only the way.
  const named =
    i.view !== null && (origin.name ?? '') !== '' && i.view.image.name.includes(origin.name ?? '');
  return [origin.kind === 'gallery' || named ? '' : place, i.view ? i.kindText : '']
    .filter((t) => t !== '')
    .join(' · ');
}

/** What the running request is called over its text. */
export function summaryOf(runFor: RunFor, f: ImageForm, fromMap: boolean): string {
  return runFor === 'edit'
    ? requestLine('edit', f.style, 0)
    : requestLine(f.kind, f.style, fromMap && f.kind !== 'texture' ? f.npcIds.length : 0);
}

/** The result's main action, in the footer: use the textured map, or show the picture. */
export function mainOf(
  v: ResultView | null,
  fromMap: boolean,
  used: boolean,
  shown: boolean,
): 'use' | 'show' | 'ask-show' | null {
  if (!v) {
    return null;
  }
  if (v.wholeMap && fromMap) {
    return used ? null : 'use';
  }
  if (shown) {
    return null;
  }
  return v.wholeMap ? 'ask-show' : 'show';
}

/** The picture of an older generated image, opened from the gallery. */
export function openedView(image: GalleryImage, chain: readonly ImageEdit[]): ResultView {
  const here = chain.find((e) => e.image?.id === image.id);
  return {
    image,
    number: here?.number ?? 0,
    caption: askedText(here?.prompt ?? ''),
    texture: false,
    wholeMap: image.showsWholeMap,
  };
}

/** One adjustment of the chain, picked to be looked at again. */
export function chainView(edit: ImageEdit & { image: GalleryImage }, fromMap: boolean): ResultView {
  return {
    image: edit.image,
    number: edit.number,
    caption: askedText(edit.prompt),
    texture: edit.image.showsWholeMap && fromMap,
    wholeMap: edit.image.showsWholeMap,
  };
}

export interface DoneInput {
  readonly image: GalleryImage;
  readonly number: number;
  readonly form: ImageForm;
  readonly edit: boolean;
  /** The adjustment asked for, when `edit`. */
  readonly instruction: string;
  readonly fromMap: boolean;
  /** "2 × 3": the grid of the drawing that went along, empty without one. */
  readonly grid: string;
}

/** The picture that just came back, with the words that say how it was asked for. */
export function doneView(i: DoneInput): ResultView {
  const whole = i.image.showsWholeMap;
  const kind: KindKey = i.form.kind;
  return {
    image: i.image,
    number: i.number,
    texture: whole && i.fromMap,
    wholeMap: whole,
    caption: i.edit
      ? resultCaption({ edit: i.instruction.trim() })
      : resultCaption({
          kind,
          fromMap: i.fromMap,
          style: i.form.style,
          npcs: whole || !i.fromMap ? 0 : i.form.npcIds.length,
          grid: i.grid,
        }),
  };
}

/** The words that name the result's way: "Ajuste", or the way of the request. */
export function resultKindOf(edit: boolean, kind: KindKey): string {
  return edit ? 'Ajuste' : KIND_LABEL[kind].long;
}

/** A refusal that says the form is out of date: the drawing and the NPC list are read again. */
export function isStale(issue: GenerateIssue): boolean {
  return (
    issue.field === 'npc_character_ids' ||
    issue.reason === ImageGenerationBlockedReason.PLAYERS_SEE_NOTHING ||
    issue.reason === ImageGenerationBlockedReason.MAP_CHANGED
  );
}

/** The refusal names a reference image (an object or a portrait). */
export function namesReference(issue: GenerateIssue): boolean {
  return issue.field === 'object_image_ids' || issue.field === 'character_image_ids';
}

/** The field the focus goes to after a refusal, when it named one. */
export function focusFieldOf(issue: GenerateIssue): 'npc' | 'refs' | undefined {
  if (issue.field === 'npc_character_ids') {
    return 'npc';
  }
  return namesReference(issue) ? 'refs' : undefined;
}
