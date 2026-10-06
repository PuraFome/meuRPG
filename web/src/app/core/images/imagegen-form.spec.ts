import { ImageAspectRatio, ImageGenerationKind, ImageStyle } from '../../../gen/meurpg/maps/v1/imagegen_pb';
import { type ImageForm, type PickableNpc, buildRequest, formProblem, npcPortraits } from './imagegen-form';

const NPCS: PickableNpc[] = [
  { characterId: 'npc-capitao', name: 'Capitão Goblin', portraitImageId: 'img-capitao' },
  { characterId: 'npc-goblin1', name: 'Goblin 1', portraitImageId: '' },
  { characterId: 'npc-vesna', name: 'Vesna, a capitã', portraitImageId: 'img-vesna' },
];

function form(partial: Partial<ImageForm> = {}): ImageForm {
  return {
    kind: 'scene',
    prompt: '  Uma cripta úmida, tochas apagadas  ',
    name: ' A cripta de Mirathel ',
    style: ImageStyle.OIL_PAINTING,
    ratio: ImageAspectRatio.IMAGE_ASPECT_RATIO_21_9,
    objectImageIds: ['img-taverna'],
    npcIds: ['npc-capitao', 'npc-goblin1'],
    ...partial,
  };
}

describe('the form → request mapping', () => {
  it('the scene art of a map (MAP_SCENE): the text trimmed, the style, the ratio, the references and the marked NPCs by id, never their portraits', () => {
    const built = buildRequest(form(), 'map-1', 'key-1');
    expect(built).toEqual({
      via: 'map',
      request: {
        mapId: 'map-1',
        kind: ImageGenerationKind.MAP_SCENE,
        idempotencyKey: 'key-1',
        prompt: 'Uma cripta úmida, tochas apagadas',
        name: 'A cripta de Mirathel',
        style: ImageStyle.OIL_PAINTING,
        aspectRatio: ImageAspectRatio.IMAGE_ASPECT_RATIO_21_9,
        objectImageIds: ['img-taverna'],
        characterImageIds: [],
        npcCharacterIds: ['npc-capitao', 'npc-goblin1'],
      },
    });
  });

  it('the isometric view takes the same fields with its own kind', () => {
    const built = buildRequest(form({ kind: 'isometric' }), 'map-1', 'k');
    expect(built.via === 'map' && built.request.kind).toBe(ImageGenerationKind.ISOMETRIC);
    expect(built.via === 'map' && built.request.npcCharacterIds).toEqual(['npc-capitao', 'npc-goblin1']);
  });

  it('the textured map sends no creature at all (no NPC, no character image) and no ratio: the server picks the model\'s closest to the grid', () => {
    const built = buildRequest(form({ kind: 'texture' }), 'map-1', 'k');
    expect(built.via).toBe('map');
    if (built.via !== 'map') {
      return;
    }
    expect(built.request.kind).toBe(ImageGenerationKind.TEXTURED_MAP);
    expect(built.request.npcCharacterIds).toEqual([]);
    expect(built.request.characterImageIds).toEqual([]);
    expect(built.request.aspectRatio).toBe(ImageAspectRatio.IMAGE_ASPECT_RATIO_UNSPECIFIED);
    // The gallery references still go: they are objects and places.
    expect(built.request.objectImageIds).toEqual(['img-taverna']);
  });

  it('from a scene or the gallery (no map) it is `GenerateSceneImage`: the text, the style, the ratio and the references only', () => {
    expect(buildRequest(form({ npcIds: [] }), null, 'k')).toEqual({
      via: 'scene',
      request: {
        idempotencyKey: 'k',
        prompt: 'Uma cripta úmida, tochas apagadas',
        name: 'A cripta de Mirathel',
        style: ImageStyle.OIL_PAINTING,
        aspectRatio: ImageAspectRatio.IMAGE_ASPECT_RATIO_21_9,
        objectImageIds: ['img-taverna'],
        characterImageIds: [],
      },
    });
  });
});

describe('what "Gerar imagem" waits for', () => {
  it('the text: dashed until there is some, and not past the limit', () => {
    expect(formProblem(form({ prompt: '   ' }))).toBe('Escreva o que a imagem mostra para gerar.');
    expect(formProblem(form())).toBeNull();
    expect(formProblem(form({ prompt: 'a'.repeat(501) }))).toBe('O texto vai até 500 caracteres.');
    expect(formProblem(form({ prompt: 'a'.repeat(500) }))).toBeNull();
  });

  it('counts characters, not bytes (an accent is one)', () => {
    expect(formProblem(form({ prompt: 'ã'.repeat(500) }))).toBeNull();
  });
});

describe('the portraits that go as character references', () => {
  it('are the marked NPCs that have one, each once; none for the textured map', () => {
    expect(npcPortraits(form(), NPCS)).toEqual(['img-capitao']);
    expect(npcPortraits(form({ npcIds: ['npc-capitao', 'npc-vesna'] }), NPCS)).toEqual(['img-capitao', 'img-vesna']);
    expect(npcPortraits(form({ kind: 'texture' }), NPCS)).toEqual([]);
  });
});
