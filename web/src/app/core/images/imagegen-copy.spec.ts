import { ImageStyle, ImageGenerationKind } from '../../../gen/meurpg/maps/v1/imagegen_pb';
import {
  NEEDS_A_GRID,
  defaultImageName,
  NEEDS_A_MAP,
  kindChoices,
  mapKindOf,
  monthName,
  remainingText,
  requestLine,
  resetText,
  resultCaption,
  whatGoesAlong,
} from './imagegen-copy';
import { imageStatus } from './imagegen-testing';

const plain = (t: string) => t.replace(/\u00a0/g, ' ');

describe('the ways to ask for a picture', () => {
  it('from a map with a grid: the three are on', () => {
    expect(kindChoices({ kind: 'map', mapId: 'm', hasGrid: true }, false).map((c) => [c.key, c.available])).toEqual([
      ['scene', true],
      ['isometric', true],
      ['texture', true],
    ]);
  });

  it('from a scene or the gallery: only "Arte da cena", the others dashed with the reason (they need a map with a grid)', () => {
    for (const kind of ['scene', 'gallery'] as const) {
      const choices = kindChoices({ kind }, false);
      expect(choices.map((c) => c.available)).toEqual([true, false, false]);
      expect(choices[1].reason).toBe(NEEDS_A_MAP);
      expect(choices[2].reason).toBe(NEEDS_A_MAP);
    }
  });

  it('from a map without a grid: the same, with the reason about the grid', () => {
    const choices = kindChoices({ kind: 'map', mapId: 'm', hasGrid: false }, false);
    expect(choices.map((c) => c.available)).toEqual([true, false, false]);
    expect(choices[1].reason).toBe(NEEDS_A_GRID);
  });

  it('the textured map is off, with its own reason, when the map\'s image is over 16 megapixels (`texture_too_large`)', () => {
    const choices = kindChoices({ kind: 'map', mapId: 'm', hasGrid: true }, true);
    expect(choices.map((c) => c.available)).toEqual([true, true, false]);
    expect(plain(choices[2].reason)).toContain('mais de 16 megapixels');
  });

  it('the wire kinds', () => {
    expect([mapKindOf('scene'), mapKindOf('isometric'), mapKindOf('texture')]).toEqual([
      ImageGenerationKind.MAP_SCENE,
      ImageGenerationKind.ISOMETRIC,
      ImageGenerationKind.TEXTURED_MAP,
    ]);
  });
});

describe('what goes along, said for the way', () => {
  it('the scene art and the isometric view start from what the players see now, and leave the doors and numbers out', () => {
    const { lead, body } = whatGoesAlong('scene', 0);
    expect(lead).toBe('A imagem parte do que os jogadores veem agora.');
    expect(body).toContain('uma porta secreta que ninguém achou é parede');
  });

  it('the textured map says the whole map goes, and lists the rooms only for a generated dungeon', () => {
    expect(whatGoesAlong('texture', 9).body).toContain('a lista das 9 salas');
    expect(whatGoesAlong('texture', 0).body).not.toContain('salas');
  });
});

describe('the month', () => {
  it('"Restam 17 de 20 imagens em outubro." and "1º de novembro"', () => {
    const status = imageStatus();
    expect(plain(remainingText(status))).toBe('Restam 17 de 20 imagens em outubro.');
    expect(plain(remainingText(imageStatus({ remaining: 1 })))).toBe('Resta 1 de 20 imagens em outubro.');
    expect(monthName('2026-12')).toBe('dezembro');
    expect(resetText(status)).toBe('1º de novembro');
  });
});

describe('the lines over a request and under a picture', () => {
  it('"Arte da cena · Pintura a óleo · 2 NPCs"', () => {
    expect(requestLine('scene', ImageStyle.OIL_PAINTING, 2)).toBe('Arte da cena · Pintura a óleo · 2 NPCs');
    expect(requestLine('texture', ImageStyle.UNSPECIFIED, 0)).toBe('O mapa com textura · Sem estilo');
    expect(requestLine('edit', ImageStyle.INK, 3)).toBe('Ajuste');
  });

  it('the caption says how the picture was made', () => {
    expect(resultCaption({ kind: 'scene', fromMap: true, style: ImageStyle.OIL_PAINTING, npcs: 2 })).toBe('Gerada a partir do mapa · Pintura a óleo · 2 NPCs');
    expect(resultCaption({ kind: 'scene', fromMap: false, style: ImageStyle.WATERCOLOR, npcs: 0 })).toBe('Gerada a partir da cena · Aquarela');
    expect(resultCaption({ kind: 'texture', fromMap: true, style: ImageStyle.INK, npcs: 0, grid: '31 × 21' })).toBe('O mapa visto de cima, com a grade por cima (31 × 21 quadrados)');
    expect(resultCaption({ edit: 'mais escura' })).toBe('Ajuste pedido: mais escura');
  });
});

describe('the default name of a picture (the players read it)', () => {
  it('names a picture by a revealed map or scene, otherwise by the way and the day, never by a hidden name (RN-10)', () => {
    const day = new Date('2026-10-06T15:00:00Z');
    const map = { kind: 'map', mapId: 'm', name: 'Masmorra de Mirathel', hasGrid: true, revealed: true } as const;
    expect(defaultImageName(map, 'scene', day)).toBe('Masmorra de Mirathel · arte da cena');
    expect(defaultImageName(map, 'isometric', day)).toBe('Masmorra de Mirathel · vista isométrica');
    expect(defaultImageName(map, 'texture', day)).toBe('Masmorra de Mirathel · mapa com textura');
    expect(defaultImageName({ kind: 'scene', name: 'Taverna do Corvo Branco', revealed: true }, 'scene', day)).toBe('Taverna do Corvo Branco');
    // A map without a grid is made as a scene art from a text: only the map's name.
    expect(defaultImageName({ ...map, hasGrid: false }, 'scene', day)).toBe('Masmorra de Mirathel');
    // Hidden from the players: the name could be a secret, so the way and the day.
    expect(defaultImageName({ ...map, revealed: false }, 'isometric', day)).toBe('Vista isométrica · 06/10');
    expect(defaultImageName({ kind: 'scene', name: 'O covil secreto' }, 'scene', day)).toBe('Arte da cena · 06/10');
    expect(defaultImageName({ kind: 'gallery' }, 'scene', day)).toBe('Arte da cena · 06/10');
  });

  it('the dashed ways use a non-breaking hyphen, so "usá-las" never breaks at the hyphen', () => {
    expect(NEEDS_A_MAP).toContain('usá\u2011las');
  });
});
