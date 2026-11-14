import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';
import { Subject } from 'rxjs';

import { FullSheetSchema } from '../../../gen/meurpg/characters/v1/characters_pb';
import type { GearVm } from '../../core/characters/gear';
import { SheetEquip, type SheetEquipHost } from './character-sheet-equip';
import type { PieceBonusData } from './piece-bonus/piece-bonus-sheet';
import type { CharacterSheetVm, GearChange } from './character-sheet.types';
import type { WeaponMarksData } from './weapon-marks/weapon-marks-sheet';

const STALE_BONUS =
  'Não foi possível salvar o bônus. A ficha mudou enquanto você editava. Nada foi salvo: feche e abra de novo.';
const STALE_MARKS =
  'Não foi possível salvar as marcas e o bônus. A ficha mudou enquanto você editava. Nada foi salvo: feche e abra de novo.';

const sword: GearVm = {
  key: 'equipment:sword',
  namePt: 'Espada',
  kind: 'weapon',
  equipped: true,
  twoHanded: false,
  armorClassIfToggled: 0,
  count: 1,
};
const shield: GearVm = { ...sword, key: 'equipment:shield', namePt: 'Escudo', kind: 'shield' };

function vmOf(over: { bonus?: number; silvered?: boolean } = {}): CharacterSheetVm {
  return {
    id: 'ch1',
    name: 'Tor',
    revision: 4,
    isMaster: false,
    sheet: { kind: 'full', weaponMarks: [] },
    fullSheet: create(FullSheetSchema, {
      weaponMarks: over.silvered ? [{ weaponKey: sword.key, silvered: true }] : [],
      itemBonuses: over.bonus
        ? [
            { key: sword.key, bonus: over.bonus },
            { key: shield.key, bonus: over.bonus },
          ]
        : [],
    }),
  } as unknown as CharacterSheetVm;
}

function setup(saveError?: unknown) {
  let vm = vmOf();
  const saved: GearChange[] = [];
  const marksOpened: WeaponMarksData[] = [];
  const bonusOpened: PieceBonusData[] = [];
  let reloads = 0;
  const host = {
    source: {
      saveGear: (_c: string, _id: string, _r: number, _n: string, _s: unknown, ch: GearChange) => {
        saved.push(ch);
        return saveError ? Promise.reject(saveError) : Promise.resolve(vm);
      },
    },
    campaignId: () => 'c1',
    characterId: () => 'ch1',
    reload: () => {
      reloads++;
      return Promise.resolve(true);
    },
    sheet: () => null,
    vm: () => vm,
    apply: () => true,
    openMarksSheet: (d: WeaponMarksData) => {
      marksOpened.push(d);
      return new Subject<boolean | undefined>();
    },
    openBonusSheet: (d: PieceBonusData) => {
      bonusOpened.push(d);
      return new Subject<boolean | undefined>();
    },
  } as unknown as SheetEquipHost;
  const equip = new SheetEquip(host);
  return {
    equip,
    saved,
    marksOpened,
    bonusOpened,
    reloads: () => reloads,
    setVm: (v: CharacterSheetVm) => (vm = v),
  };
}

const marks = { silvered: false, adamantine: false, magical: false };

describe('SheetEquip saves from the piece sheets', () => {
  it('saves a bonus over the sheet it opened with', async () => {
    const t = setup();
    t.equip.openPiece(shield);
    expect(await t.bonusOpened[0].save(2)).toBe('');
    expect(t.saved).toEqual([{ bonus: { key: shield.key, bonus: 2 } }]);
    expect(t.reloads()).toBe(0);
  });

  it('answers a stale revision with the sheet’s own words and reads the sheet again', async () => {
    const bonus = setup(new ConnectError('stale', Code.Aborted));
    bonus.equip.openPiece(shield);
    expect(await bonus.bonusOpened[0].save(1)).toBe(STALE_BONUS);
    expect(bonus.reloads()).toBe(1);

    const weapon = setup(new ConnectError('stale', Code.Aborted));
    weapon.equip.openPiece(sword);
    expect(await weapon.marksOpened[0].save({ marks, bonus: 1 })).toBe(STALE_MARKS);
    expect(weapon.reloads()).toBe(1);
  });

  it('keeps the usual words for any other refusal and does not read again', async () => {
    const t = setup(new ConnectError('x', Code.PermissionDenied));
    t.equip.openPiece(shield);
    expect(await t.bonusOpened[0].save(1)).toBe('Você não tem permissão para fazer isso.');
    expect(t.reloads()).toBe(0);
  });

  it('refuses a bonus that changed since the sheet opened: nothing is saved, the sheet is read again', async () => {
    const t = setup();
    t.equip.openPiece(shield);
    t.setVm(vmOf({ bonus: 3 }));
    expect(await t.bonusOpened[0].save(1)).toBe(STALE_BONUS);
    expect(t.saved).toEqual([]);
    expect(t.reloads()).toBe(1);
  });

  it('refuses marks that changed since the sheet opened', async () => {
    const t = setup();
    t.equip.openPiece(sword);
    t.setVm(vmOf({ silvered: true }));
    expect(await t.marksOpened[0].save({ marks, bonus: 0 })).toBe(STALE_MARKS);
    expect(t.saved).toEqual([]);
    expect(t.reloads()).toBe(1);
  });
});
