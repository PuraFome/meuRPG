import { Code, ConnectError } from '@connectrpc/connect';
import { signal } from '@angular/core';
import { of } from 'rxjs';

import type { AttunementEntryVm, AttunementVm } from '../../../core/characters/attunement';
import type { CharacterSheetVm } from '../character-sheet.types';
import { SheetMagicItems, type SheetMagicItemsHost } from './sheet-magic-items';

const entry = (over: Partial<AttunementEntryVm>): AttunementEntryVm => ({
  id: 'm1',
  key: 'cloak',
  namePt: 'Manto élfico',
  kind: 'magic',
  attuned: false,
  asks: true,
  restrictionPt: '',
  restriction: 'none',
  sameItemAttuned: false,
  ...over,
});

const vm = {
  id: 'c1',
  name: 'Mirta',
  revision: 7,
  isMaster: false,
  state: 'alive',
  fullSheet: { marker: 'full' },
} as unknown as CharacterSheetVm;

function setup(entries: AttunementEntryVm[] = [entry({})]) {
  const attunement = signal<AttunementVm>({
    entries,
    attuned: entries.filter((e) => e.asks && e.attuned).length,
  });
  const setMagicItems = vi.fn().mockResolvedValue({ revision: 8 });
  const apply = vi.fn();
  const reload = vi.fn();
  const openPicker = vi.fn();
  const openRemove = vi.fn();
  const openAttunement = vi.fn().mockReturnValue(of(undefined));
  const host = {
    source: { setMagicItems },
    campaignId: () => 'camp',
    characterId: () => 'c1',
    reload,
    apply,
    vm: () => vm,
    attunement,
    revision: signal(7),
    windowOpen: signal(true),
    openAttunement,
    openPicker,
    openRemove,
  } as unknown as SheetMagicItemsHost;
  return {
    magic: new SheetMagicItems(host),
    setMagicItems,
    apply,
    reload,
    openPicker,
    openRemove,
    openAttunement,
  };
}

describe('SheetMagicItems', () => {
  it('adds an item as a full-sheet save at the held revision, keeping the ids of the others', async () => {
    const t = setup();
    t.openPicker.mockReturnValue(of(true));
    t.magic.openPicker();
    const add = t.openPicker.mock.calls[0][0].add as (key: string) => Promise<string>;
    expect(await add('ring')).toBe('');
    expect(t.setMagicItems).toHaveBeenCalledWith('camp', 'c1', 7, 'Mirta', vm.fullSheet, [
      { id: 'm1', key: 'cloak' },
      { id: '', key: 'ring' },
    ]);
    expect(t.apply).toHaveBeenCalledWith('c1', { revision: 8 });
    expect(t.magic.notice()).toBe('Item mágico adicionado.');
  });

  it('answers a stale sheet in words and applies nothing', async () => {
    const t = setup();
    t.setMagicItems.mockRejectedValue(new ConnectError('stale', Code.Aborted));
    t.openPicker.mockReturnValue(of(undefined));
    t.magic.openPicker();
    const add = t.openPicker.mock.calls[0][0].add as (key: string) => Promise<string>;
    expect(await add('ring')).not.toBe('');
    expect(t.apply).not.toHaveBeenCalled();
    expect(t.magic.notice()).toBe('');
  });

  it('removes only the entry that was named, and counts an attuned one that asks', async () => {
    const t = setup([entry({ attuned: true }), entry({ id: 'm2', key: 'ring', namePt: 'Anel' })]);
    t.openRemove.mockReturnValue(of(true));
    t.magic.openRemove(entry({ attuned: true }));
    const data = t.openRemove.mock.calls[0][0];
    expect(data.attunedBefore).toBe(1);
    expect(await data.remove()).toBe('');
    expect(t.setMagicItems.mock.calls[0][5]).toEqual([{ id: 'm2', key: 'ring' }]);
    expect(t.magic.notice()).toBe('Manto élfico saiu da ficha.');
  });

  it('does not count an item that is not attuned', () => {
    const t = setup();
    t.openRemove.mockReturnValue(of(undefined));
    t.magic.openRemove(entry({}));
    expect(t.openRemove.mock.calls[0][0].attunedBefore).toBeNull();
  });

  it('opens the attunement sheet for the character and reads the sheet again after a change', () => {
    const t = setup();
    t.magic.openAttunement();
    const data = t.openAttunement.mock.calls[0][0];
    expect(data).toMatchObject({
      campaignId: 'camp',
      characterId: 'c1',
      isMaster: false,
      dead: false,
    });
    data.changed();
    expect(t.reload).toHaveBeenCalled();
  });
});
