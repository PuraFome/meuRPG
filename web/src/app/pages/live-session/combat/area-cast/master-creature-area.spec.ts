import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { By } from '@angular/platform-browser';
import { create } from '@bufbuild/protobuf';

import {
  AreaPlacement,
  CreatureActionResultSchema,
  CombatantKind,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  Ability,
  CreatureAreaShape,
  SpellSaveSuccess,
} from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { fakeCombatClient } from '../../../../core/combat/combat-client-testing';
import { combatant, creatureAction, encounter } from '../../../../core/combat/combat-testing';
import { AreaMap } from '../../../../shared/area-picker/area-map';
import { settle } from '../../../../../test-settle';
import { MasterCreatureArea, type MasterCreatureAreaData } from './master-creature-area';

const plain = (t: string | null | undefined) => (t ?? '').replace(/\s+/g, ' ').trim();

const hound = combatant({ id: 'hound', label: 'Cão Infernal 1', col: 8, row: 1 });
const toren = combatant({ id: 't', label: 'Toren', kind: CombatantKind.PLAYER, col: 5, row: 4 });

const breath = creatureAction({
  key: 'monster:hell-hound#fire-breath',
  name: 'Fire Breath',
  namePt: 'Sopro de Fogo',
  area: { shape: CreatureAreaShape.CONE, sizeFt: 15, widthFt: 0 },
  save: { ability: Ability.DEXTERITY, dc: 13, onSuccess: SpellSaveSuccess.HALF, damage: [] },
} as never);

async function open(placement = AreaPlacement.DIRECTION, fail = false) {
  TestBed.resetTestingModule();
  const result = create(CreatureActionResultSchema, { castId: 'cast' });
  const useCreatureAction = fail
    ? vi.fn().mockRejectedValue(new Error('lost'))
    : vi.fn().mockResolvedValue({ encounter: encounter(), result, area: null });
  const close = vi.fn();
  const apply = vi.fn();
  const data: MasterCreatureAreaData = {
    campaignId: 'c',
    encounterId: 'enc',
    actor: hound,
    action: breath,
    placement,
    targetIds: [],
    map: {
      image: { url: 'cave.png', width: 1200, height: 800 },
      mapName: 'A caverna',
      columns: 12,
      rows: 8,
      layers: null,
      fog: null,
    },
    state: { encounter: signal(encounter({ combatants: [hound, toren] })), apply } as never,
  };
  TestBed.configureTestingModule({
    providers: [
      { provide: CombatClient, useValue: fakeCombatClient({ useCreatureAction }) },
      { provide: MAT_DIALOG_DATA, useValue: data },
      { provide: MatDialogRef, useValue: { close } },
    ],
  });
  const fixture = TestBed.createComponent(MasterCreatureArea);
  await settle(fixture);
  const el = fixture.nativeElement as HTMLElement;
  const use = () => el.querySelector<HTMLButtonElement>('button[aria-label="Usar Sopro de Fogo"]')!;
  const map = fixture.debugElement.query(By.directive(AreaMap)).componentInstance as AreaMap;
  return { fixture, el, use, map, useCreatureAction, close, apply };
}

describe('MasterCreatureArea: the master places the area of a creature action', () => {
  it('names the creature and the action and the shape, and keeps "Usar" off until a direction is placed', async () => {
    const { el, use } = await open();
    expect(plain(el.querySelector('.frame__title')?.textContent)).toBe(
      'Cão Infernal 1 usa Sopro de Fogo',
    );
    expect(plain(el.textContent)).toContain('cone de 4,5');
    expect(plain(el.textContent)).toContain('Para onde?');
    expect(use().classList.contains('mr-button--off')).toBe(true);
    use().click();
  });

  it('sends the placed direction with the action key and no target, applies the combat and closes with the result', async () => {
    const { fixture, use, map, useCreatureAction, close, apply } = await open();
    map.place.emit({ kind: 'direction', direction: { dx: 1, dy: 1 } });
    await settle(fixture);
    use().click();
    await settle(fixture);
    expect(useCreatureAction).toHaveBeenCalledWith(
      'c',
      'enc',
      'hound',
      'monster:hell-hound#fire-breath',
      [],
      { direction: { dx: 1, dy: 1 } },
    );
    expect(apply).toHaveBeenCalled();
    expect(close).toHaveBeenCalledWith(
      expect.objectContaining({
        action: breath,
        result: expect.objectContaining({ castId: 'cast' }),
      }),
    );
  });

  it('sends a point for a sphere', async () => {
    const { fixture, use, map, useCreatureAction } = await open(AreaPlacement.POINT);
    map.place.emit({ kind: 'point', square: { col: 6, row: 4 } });
    await settle(fixture);
    use().click();
    await settle(fixture);
    expect(useCreatureAction.mock.calls[0][5]).toEqual({ origin: { col: 6, row: 4 } });
  });

  it('says the refusal under the map and keeps the dialog open', async () => {
    const { fixture, el, use, map, close } = await open(AreaPlacement.DIRECTION, true);
    map.place.emit({ kind: 'direction', direction: { dx: 0, dy: 1 } });
    await settle(fixture);
    use().click();
    await settle(fixture);
    expect(el.querySelector('[role="alert"]')).not.toBeNull();
    expect(close).not.toHaveBeenCalled();
  });

  it('closes with nothing on "Cancelar"', async () => {
    const { el, close } = await open();
    [...el.querySelectorAll('button')].find((b) => plain(b.textContent) === 'Cancelar')!.click();
    expect(close).toHaveBeenCalledWith(null);
  });
});
