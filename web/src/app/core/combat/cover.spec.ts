import { describe, expect, it } from 'vitest';

import { CombatantKind, CombatantSide, CoverDegree, CoverSource } from '../../../gen/meurpg/play/v1/combat_pb';
import { coverText, listing, markTags } from './cover';

describe('cover on screen', () => {
  it('names the degree and where it comes from, never an object', () => {
    expect(coverText(CoverDegree.HALF, CoverSource.MAP)).toBe('Meia cobertura (do mapa)');
    expect(coverText(CoverDegree.THREE_QUARTERS, CoverSource.MARK)).toBe('Três quartos (marcada pelo mestre)');
    expect(coverText(CoverDegree.NONE, CoverSource.UNSPECIFIED)).toBe('');
    expect(coverText(CoverDegree.UNSPECIFIED, CoverSource.UNSPECIFIED)).toBe('');
  });

  it('lists a covered target with its pictogram', () => {
    expect(listing({ cover: CoverDegree.HALF, coverSource: CoverSource.MAP, untargetable: false })).toEqual({
      kind: 'listed',
      text: 'Meia cobertura (do mapa)',
      mark: 'half',
    });
    expect(listing({ cover: CoverDegree.NONE, coverSource: CoverSource.UNSPECIFIED, untargetable: false })).toEqual({
      kind: 'listed',
      text: '',
      mark: null,
    });
  });

  it('disables a target the master marked total, with the reason, and leaves out a wall', () => {
    expect(listing({ cover: CoverDegree.TOTAL, coverSource: CoverSource.MARK, untargetable: true })).toEqual({
      kind: 'blocked',
      text: 'Cobertura total (marcada pelo mestre): não pode ser alvo',
    });
    expect(listing({ cover: CoverDegree.TOTAL, coverSource: CoverSource.MAP, untargetable: true })).toEqual({ kind: 'left-out' });
  });

  it('tags an ally and the master\'s manual mark, and nothing else', () => {
    expect(markTags({ kind: CombatantKind.NPC, side: CombatantSide.PARTY, coverMark: CoverDegree.THREE_QUARTERS })).toEqual([
      'Aliado',
      'Três quartos · marcada pelo mestre',
    ]);
    expect(markTags({ kind: CombatantKind.NPC, side: CombatantSide.ENEMY, coverMark: CoverDegree.NONE })).toEqual([]);
    // A player's character is always the party: not an "Aliado".
    expect(markTags({ kind: CombatantKind.PLAYER, side: CombatantSide.PARTY, coverMark: CoverDegree.UNSPECIFIED })).toEqual([]);
  });
});
