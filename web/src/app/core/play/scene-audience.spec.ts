import { create } from '@bufbuild/protobuf';

import { SceneAudienceViewSchema } from '../../../gen/meurpg/play/v1/scene_pb';
import {
  choosablePlayers,
  imageChooserLine,
  insideSummary,
  outsideLine,
  sceneChooserLine,
  shownAudienceLine,
} from './scene-audience';

function player(userId: string, characterName: string) {
  return { userId, characterId: `c-${userId}`, characterName, joinedBySelf: false };
}

function audience(inside: string[], outside: string[], everyone = false) {
  return create(SceneAudienceViewSchema, {
    everyone,
    inside: inside.map((n) => player(n.toLowerCase(), n)),
    outside: outside.map((n) => player(n.toLowerCase(), n)),
  });
}

describe('scene audience words (MR-015, MR-028)', () => {
  it('lists the players of the table, those in the scene first', () => {
    const rows = choosablePlayers(audience(['Pensantus', 'Brisa'], ['Toren']));
    expect(rows.map((r) => `${r.name}: ${r.sub}`)).toEqual([
      'Pensantus: na cena',
      'Brisa: na cena',
      'Toren: fora da cena',
    ]);
    expect(choosablePlayers(null)).toEqual([]);
  });

  it('names up to three players inside and counts more', () => {
    expect(insideSummary(audience(['Pensantus', 'Brisa'], ['Toren']))).toEqual({
      chips: ['Pensantus', 'Brisa'],
      count: '',
    });
    const many = insideSummary(audience(['A', 'B', 'C', 'D', 'E', 'F'], ['Sálvia', 'Jabuti']));
    expect(many.count).toBe('6 de 8 jogadores');
  });

  it('says who is out of the scene', () => {
    expect(outsideLine(audience(['A'], ['Toren']))).toBe('Toren não está nesta cena.');
    expect(outsideLine(audience(['A'], ['Sálvia', 'Jabuti']))).toBe(
      'Fora da cena: Sálvia e Jabuti.',
    );
    expect(outsideLine(audience(['A', 'B'], [], true))).toBe('');
  });

  it('says the live lines of the two choosers', () => {
    expect(sceneChooserLine([], false)).toBe('Ninguém foi escolhido. Marque quem está na cena.');
    expect(sceneChooserLine(['Brisa'], false)).toBe('Brisa está na cena.');
    expect(sceneChooserLine(['Brisa', 'Toren'], false)).toBe('Brisa e Toren estão na cena.');
    expect(sceneChooserLine(['Brisa'], true)).toBe('Toda a mesa está na cena.');
    expect(imageChooserLine(['Pensantus', 'Brisa'], false)).toBe(
      'Pensantus e Brisa veem a imagem.',
    );
    expect(imageChooserLine(['Brisa'], false)).toBe('Brisa vê a imagem.');
    expect(imageChooserLine([], true)).toBe('Toda a mesa vê a imagem.');
  });

  describe('the line under the images', () => {
    const partial = audience(['Pensantus', 'Brisa'], ['Toren']);

    it('says "Os jogadores veem" when the scene is open to everyone or none is', () => {
      const whole = audience(['Pensantus', 'Brisa', 'Toren'], [], true);
      expect(shownAudienceLine({ kind: 'whole', userIds: [] }, whole)).toBe('Os jogadores veem');
      expect(shownAudienceLine(undefined, null)).toBe('Os jogadores veem');
    });

    it('says "Toda a mesa vê" for the whole table in a partial scene', () => {
      expect(shownAudienceLine({ kind: 'whole', userIds: [] }, partial)).toBe('Toda a mesa vê');
    });

    it("lists the scene's players, or counts them past three", () => {
      expect(shownAudienceLine({ kind: 'scene', userIds: [] }, partial)).toBe(
        'Pensantus e Brisa veem',
      );
      const big = audience(['A', 'B', 'C', 'D', 'E', 'F'], ['G', 'H']);
      expect(shownAudienceLine({ kind: 'scene', userIds: [] }, big)).toBe('6 de 8 jogadores veem');
    });

    it('lists the players chosen', () => {
      expect(shownAudienceLine({ kind: 'chosen', userIds: ['brisa'] }, partial)).toBe(
        'Só Brisa vê',
      );
      expect(shownAudienceLine({ kind: 'chosen', userIds: ['brisa', 'toren'] }, partial)).toBe(
        'Brisa e Toren veem',
      );
    });
  });
});
