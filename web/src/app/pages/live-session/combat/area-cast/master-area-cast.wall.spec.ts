import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { By } from '@angular/platform-browser';
import { create } from '@bufbuild/protobuf';

import { HiddenAreaHitRule } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  AreaPlacement,
  PreviewSpellAreaResponseSchema,
  SpellTargetsSchema,
  WallSide,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { ActionEconomy, SpellAreaShape } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { TableRulesClient } from '../../../../core/campaigns/table-rules';
import { CombatClient } from '../../../../core/combat/combat-client';
import { fakeCombatClient } from '../../../../core/combat/combat-client-testing';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { SpellCatalog } from '../../../../core/combat/spell-catalog';
import { AreaMap } from '../../../../shared/area-picker/area-map';
import { MasterAreaCast, type MasterAreaCastData } from './master-area-cast';

const plain = (t: string | null | undefined) =>
  (t ?? '').replace(/ /g, ' ').replace(/\s+/g, ' ').trim();

const zuk = combatant({ id: 'z', label: 'Zuk', col: 8, row: 1 });
const goblin = combatant({ id: 'g1', label: 'Goblin 1', col: 6, row: 5 });

const details = {
  spell: { level: 4 },
  damage: [{ damageTypeKey: 'fire', bySlotLevel: { 4: '5d8' }, byCharacterLevel: {} }],
  healBySlotLevel: {},
  higherLevel: [],
};

async function open() {
  TestBed.resetTestingModule();
  const castSpell = vi.fn().mockResolvedValue({ encounter: encounter(), cast: { targets: [] } });
  const previewSpellArea = vi.fn().mockResolvedValue(
    create(PreviewSpellAreaResponseSchema, {
      origin: { col: 6, row: 4 },
      squares: [{ col: 6, row: 4 }],
      burningSquares: [{ col: 6, row: 5 }],
      coverCounts: true,
      targets: [{ combatantId: 'g1', label: 'Goblin 1' }],
    }),
  );
  const data: MasterAreaCastData = {
    campaignId: 'c',
    encounterId: 'enc',
    caster: zuk,
    spellKey: 'spell:wall-of-fire',
    name: 'Muralha de Fogo',
    level: 4,
    slot: { level: 4, pact: false },
    economy: ActionEconomy.ACTION,
    targets: create(SpellTargetsSchema, {
      placement: AreaPlacement.POINT,
      areaShape: SpellAreaShape.LINE,
      areaSizeFt: 60,
      areaWidthFt: 5,
      rangeFt: 120,
    }),
    map: {
      image: { url: 'cave.png', width: 1200, height: 800 },
      mapName: 'A caverna',
      columns: 12,
      rows: 8,
      layers: null,
      fog: null,
    },
    state: {
      encounter: signal(encounter({ combatants: [zuk, goblin] })),
      apply: vi.fn(),
    } as never,
  };
  TestBed.configureTestingModule({
    providers: [
      { provide: CombatClient, useValue: fakeCombatClient({ castSpell, previewSpellArea }) },
      { provide: SpellCatalog, useValue: { details: () => Promise.resolve(details) } },
      {
        provide: TableRulesClient,
        useValue: {
          get: () => Promise.resolve({ saved: { hiddenAreaHits: HiddenAreaHitRule.REVEAL } }),
        },
      },
      { provide: MAT_DIALOG_DATA, useValue: data },
      { provide: MatDialogRef, useValue: { close: vi.fn() } },
    ],
  });
  const fixture = TestBed.createComponent(MasterAreaCast);
  const settle = async () => {
    for (let i = 0; i < 3; i++) {
      fixture.detectChanges();
      await fixture.whenStable();
    }
    fixture.detectChanges();
  };
  await settle();
  const el = fixture.nativeElement as HTMLElement;
  const button = (label: string) =>
    Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find(
      (b) => b.getAttribute('aria-label') === label || plain(b.textContent).includes(label),
    )!;
  const map = fixture.debugElement.query(By.directive(AreaMap)).componentInstance as AreaMap;
  return { el, button, settle, castSpell, previewSpellArea, map };
}

describe('MasterAreaCast: the master lays a Muralha de Fogo for an NPC', () => {
  it('shows "Onde fica" with the wall controls and sends the wall with every preview', async () => {
    const { el, button, settle, previewSpellArea, map } = await open();
    expect(plain(el.textContent)).toContain('Onde fica');
    expect(el.querySelector('[role="radiogroup"][aria-label="Forma da muralha"]')).not.toBeNull();
    expect(plain(el.textContent)).toContain('5d8 de fogo');
    map.place.emit({ kind: 'point', square: { col: 6, row: 4 } });
    await settle();
    expect(previewSpellArea).toHaveBeenLastCalledWith(
      'c',
      'enc',
      'z',
      'spell:wall-of-fire',
      { level: 4, pact: false },
      { origin: { col: 6, row: 4 } },
      '',
      { ring: false, direction: { dx: 1, dy: 0 }, lengthFt: 60, side: WallSide.RIGHT },
    );
    button('Para o oeste').click();
    await settle();
    expect((previewSpellArea.mock.calls.at(-1) as unknown[])[7]).toMatchObject({
      direction: { dx: -1, dy: 0 },
    });
  });

  it('goes to the list with "Próximo: quem está na área" and casts with the wall', async () => {
    const { el, button, settle, castSpell, map } = await open();
    map.place.emit({ kind: 'point', square: { col: 6, row: 4 } });
    await settle();
    button('Próximo: quem está na área').click();
    await settle();
    expect(plain(el.textContent)).toContain('Quem está na área');
    button('Conjurar Muralha de Fogo').click();
    await settle();
    const extras = castSpell.mock.calls[0][12] as Record<string, unknown>;
    expect(extras['area']).toEqual({ origin: { col: 6, row: 4 } });
    expect(extras['wall']).toEqual({
      ring: false,
      direction: { dx: 1, dy: 0 },
      lengthFt: 60,
      side: WallSide.RIGHT,
    });
  });
});
