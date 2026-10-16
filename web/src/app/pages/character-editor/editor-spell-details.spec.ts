import type { MatBottomSheet } from '@angular/material/bottom-sheet';
import type { MatDialog } from '@angular/material/dialog';
import { signal } from '@angular/core';

import type { SpellDetailsData } from '../../shared/spell-details/spell-details';
import type { SpellDetailsVm } from '../../shared/spell-details/spell-details.types';
import type { SpellOptionVm } from './character-editor.types';
import type { PageState } from './editor-state';
import { EditorSpellDetails } from './editor-spell-details';

const spell = { key: 'spell:light', namePt: 'Luz' } as SpellOptionVm;

describe('EditorSpellDetails', () => {
  function setup(state: PageState, loadSpellDetails = vi.fn()) {
    // Phone or not, one of the two opens it: the spec reads the data either of them is given.
    const dialogOpen = vi.fn();
    const sheetOpen = vi.fn();
    const details = new EditorSpellDetails({
      source: { loadSpellDetails },
      state: signal(state),
      dialog: { open: dialogOpen } as unknown as MatDialog,
      bottomSheet: { open: sheetOpen } as unknown as MatBottomSheet,
    });
    const opened = {
      get called() {
        return dialogOpen.mock.calls.length + sheetOpen.mock.calls.length > 0;
      },
      data: () =>
        ((dialogOpen.mock.calls[0] ?? sheetOpen.mock.calls[0])[1] as { data: SpellDetailsData })
          .data,
    };
    return { details, opened, loadSpellDetails };
  }

  it('opens nothing while the page has no lists', () => {
    const { details, opened } = setup({ status: 'loading', title: 'Ficha' });
    details.describe(spell);
    expect(opened.called).toBe(false);
  });

  it('reads a description once, and asks again after a failed read', async () => {
    const loadSpellDetails = vi
      .fn()
      .mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValue({ key: 'spell:light' } as SpellDetailsVm);
    const { details, opened } = setup(
      { status: 'ready', campaignId: 'camp' } as unknown as PageState,
      loadSpellDetails,
    );
    details.describe(spell);
    const data = opened.data();
    expect(data.namePt).toBe('Luz');
    await expect(data.load()).rejects.toThrow('offline');
    await Promise.resolve();
    const again = await data.load();
    expect(again.key).toBe('spell:light');
    await data.load();
    expect(loadSpellDetails).toHaveBeenCalledTimes(2);
    expect(loadSpellDetails).toHaveBeenCalledWith('camp', 'spell:light');
  });
});
