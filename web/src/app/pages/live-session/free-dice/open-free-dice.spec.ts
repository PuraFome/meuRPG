import type { MatBottomSheet } from '@angular/material/bottom-sheet';
import type { MatDialog } from '@angular/material/dialog';
import { of } from 'rxjs';

import { openFreeDice, type FreeDiceSheetData } from './free-dice-sheet';

describe('openFreeDice', () => {
  const original = window.matchMedia;
  afterEach(() => {
    window.matchMedia = original;
  });

  it('opens the phone bottom sheet with its own panel class, not the shared tall one', () => {
    window.matchMedia = ((query: string) => ({ matches: true, media: query })) as never;
    const open = vi.fn().mockReturnValue({ afterDismissed: () => of(undefined) });
    openFreeDice(
      {} as MatDialog,
      { open } as unknown as MatBottomSheet,
      {} as unknown as FreeDiceSheetData,
    ).subscribe();
    const config = open.mock.lastCall?.[1] as { panelClass: string };
    expect(config.panelClass).toBe('mr-sheet-dice');
  });
});
