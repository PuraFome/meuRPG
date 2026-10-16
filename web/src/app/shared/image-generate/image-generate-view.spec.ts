import { create } from '@bufbuild/protobuf';

import { GalleryImageSchema } from '../../../gen/meurpg/maps/v1/gallery_pb';
import {
  ImageEditSchema,
  ImageGenerationBlockedReason,
  ImageGenerationStatusSchema,
  ImageStyle,
} from '../../../gen/meurpg/maps/v1/imagegen_pb';
import type { ImageForm } from '../../core/images/imagegen-form';
import type { ReferenceView } from './generate-form';
import type { ResultView } from './generate-result';
import {
  chainView,
  doneView,
  editWhyOf,
  focusFieldOf,
  isStale,
  mainOf,
  namesReference,
  openedView,
  resultKindOf,
  subtitleOf,
  summaryOf,
  whyOf,
} from './image-generate-view';

const form: ImageForm = {
  kind: 'scene',
  prompt: 'Uma taverna',
  name: 'Taverna',
  style: ImageStyle.OIL_PAINTING,
  ratio: 0,
  objectImageIds: [],
  npcIds: ['n1', 'n2'],
};
const on = create(ImageGenerationStatusSchema, {
  enabled: true,
  remaining: 3,
  monthlyLimit: 10,
  maxPromptCharacters: 2000,
});
const image = (over: { name?: string; showsWholeMap?: boolean } = {}) =>
  create(GalleryImageSchema, { id: 'i1', name: 'Taverna', ...over });
const reference = (playersSee: boolean) => ({ playersSee }) as ReferenceView;
const view = (over: Partial<ResultView> = {}): ResultView => ({
  image: image(),
  number: 1,
  caption: '',
  texture: false,
  wholeMap: false,
  ...over,
});
const why = (over: Partial<Parameters<typeof whyOf>[0]> = {}) =>
  whyOf({
    status: on,
    form,
    mapId: null,
    referenceState: 'none',
    reference: null,
    ...over,
  });

describe('whyOf', () => {
  it('is empty when the form is good and the server is on', () => {
    expect(why()).toBe('');
  });

  it('says the feature is off, then that the month is spent', () => {
    expect(why({ status: create(ImageGenerationStatusSchema, { enabled: false }) })).toBe(
      'A geração de imagens não está ligada neste servidor.',
    );
    expect(
      why({ status: create(ImageGenerationStatusSchema, { enabled: true, remaining: 0 }) }),
    ).not.toBe('');
  });

  it('from a map: waits for the drawing, and for players who see something', () => {
    expect(why({ mapId: 'm', referenceState: 'loading' })).toBe(
      'Montando o desenho que vai junto…',
    );
    expect(why({ mapId: 'm', referenceState: 'ready', reference: reference(false) })).not.toBe('');
    // The textured map does not depend on who the players see.
    expect(
      why({
        mapId: 'm',
        form: { ...form, kind: 'texture' },
        referenceState: 'ready',
        reference: reference(false),
      }),
    ).toBe('');
  });

  it('asks for the text last', () => {
    expect(why({ form: { ...form, prompt: '' } })).not.toBe('');
  });
});

describe('editWhyOf', () => {
  it('asks for the text, or says the month is spent', () => {
    expect(editWhyOf(on, '  ')).toBe('Escreva o ajuste que você quer pedir.');
    expect(editWhyOf(on, 'mais luz')).toBe('');
    expect(
      editWhyOf(create(ImageGenerationStatusSchema, { enabled: true, remaining: 0 }), 'x'),
    ).not.toBe('');
    expect(editWhyOf(null, 'x')).toBe('');
  });
});

describe('subtitleOf', () => {
  const origin = { kind: 'map', name: 'Masmorra', mapId: 'm' } as const;
  const base = { origin, stage: 'form', sending: false, view: null, kindText: '' } as const;

  it('names the place, or the scene', () => {
    expect(subtitleOf(base)).toBe('Masmorra');
    expect(subtitleOf({ ...base, origin: { kind: 'scene', name: 'Taverna' } })).toBe(
      'Cena: Taverna',
    );
  });

  it('says what the wait is', () => {
    expect(subtitleOf({ ...base, stage: 'running', sending: true })).toBe(
      'Antes de o pedido sair do servidor',
    );
    expect(subtitleOf({ ...base, stage: 'running' })).toBe('O servidor está gerando a imagem');
  });

  it("on the result says the place and the way, but not the place the image's name already holds", () => {
    const result = { ...base, stage: 'result', kindText: 'Ajuste' } as const;
    expect(subtitleOf({ ...result, view: view({ image: image({ name: 'Cripta' }) }) })).toBe(
      'Masmorra · Ajuste',
    );
    expect(subtitleOf({ ...result, view: view({ image: image({ name: 'Masmorra 2' }) }) })).toBe(
      'Ajuste',
    );
    expect(
      subtitleOf({ ...result, origin: { kind: 'gallery' }, view: view(), kindText: 'Imagem' }),
    ).toBe('Imagem');
    expect(subtitleOf(result)).toBe('Masmorra');
  });
});

describe('summaryOf, mainOf and resultKindOf', () => {
  it('counts the NPCs only for a request from a map that is not the textured one', () => {
    expect(summaryOf('generate', form, true)).toBe(summaryOf('generate', form, true));
    expect(summaryOf('generate', form, true)).not.toBe(summaryOf('generate', form, false));
    expect(summaryOf('generate', { ...form, kind: 'texture' }, true)).toBe(
      summaryOf('generate', { ...form, kind: 'texture', npcIds: [] }, true),
    );
    expect(summaryOf('edit', form, true)).toBe(summaryOf('edit', { ...form, npcIds: [] }, false));
  });

  it("chooses the footer's main action", () => {
    expect(mainOf(null, true, false, false)).toBeNull();
    expect(mainOf(view({ wholeMap: true }), true, false, false)).toBe('use');
    expect(mainOf(view({ wholeMap: true }), true, true, false)).toBeNull();
    expect(mainOf(view({ wholeMap: true }), false, false, false)).toBe('ask-show');
    expect(mainOf(view(), true, false, false)).toBe('show');
    expect(mainOf(view(), true, false, true)).toBeNull();
  });

  it('names the way of a result', () => {
    expect(resultKindOf(true, 'scene')).toBe('Ajuste');
    expect(resultKindOf(false, 'scene')).not.toBe('Ajuste');
  });
});

describe('the views of a picture', () => {
  it('opens an older image on its place in the chain', () => {
    const chain = [create(ImageEditSchema, { number: 2, prompt: 'mais luz', image: image() })];
    expect(openedView(image({ showsWholeMap: true }), chain)).toMatchObject({
      number: 2,
      caption: 'Pedido: mais luz',
      texture: false,
      wholeMap: true,
    });
    expect(openedView(image(), [])).toMatchObject({ number: 0, caption: '' });
  });

  it('puts the grid over an adjustment of the whole map from a map only', () => {
    const edit = create(ImageEditSchema, {
      number: 3,
      prompt: '',
      image: image({ showsWholeMap: true }),
    });
    const withImage = { ...edit, image: edit.image! };
    expect(chainView(withImage, true)).toMatchObject({ number: 3, caption: '', texture: true });
    expect(chainView(withImage, false).texture).toBe(false);
  });

  it('words the picture that came back by how it was asked for', () => {
    const base = {
      image: image(),
      number: 1,
      form,
      edit: false,
      instruction: '',
      fromMap: true,
      grid: '',
    };
    expect(doneView(base)).toMatchObject({ number: 1, texture: false, wholeMap: false });
    expect(doneView({ ...base, image: image({ showsWholeMap: true }) })).toMatchObject({
      texture: true,
      wholeMap: true,
    });
    expect(doneView({ ...base, fromMap: false }).texture).toBe(false);
    expect(doneView({ ...base, edit: true, instruction: ' mais luz ' }).caption).toContain(
      'mais luz',
    );
  });
});

describe('the refusals that say the form is out of date', () => {
  const issue = (field: string | null, reason: ImageGenerationBlockedReason | null = null) => ({
    text: '',
    field,
    reason,
    status: null,
  });

  it('are stale for an NPC, nobody to see, or a map that changed', () => {
    expect(isStale(issue('npc_character_ids'))).toBe(true);
    expect(isStale(issue(null, ImageGenerationBlockedReason.PLAYERS_SEE_NOTHING))).toBe(true);
    expect(isStale(issue(null, ImageGenerationBlockedReason.MAP_CHANGED))).toBe(true);
    expect(isStale(issue('prompt'))).toBe(false);
  });

  it('name the field the focus goes to', () => {
    expect(focusFieldOf(issue('npc_character_ids'))).toBe('npc');
    expect(focusFieldOf(issue('object_image_ids'))).toBe('refs');
    expect(focusFieldOf(issue('character_image_ids'))).toBe('refs');
    expect(namesReference(issue('character_image_ids'))).toBe(true);
    expect(focusFieldOf(issue('prompt'))).toBeUndefined();
  });
});
