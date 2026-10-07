import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';

import { openSpellDetails } from './open-spell-details';
import { SpellDetails } from './spell-details';
import type { SpellDetailsVm } from './spell-details.types';

const SLEEP = {
  key: 'spell:sleep',
  namePt: 'Sono',
  nameEn: 'Sleep',
  level: 1,
  schoolNamePt: 'Encantamento',
  ritual: false,
  concentration: false,
  castingTime: { amount: 1, unit: 'action', trigger: '', raw: '1 action' },
  range: { kind: 'ranged', distanceFt: 90, raw: '90 feet' },
  components: { verbal: true, somatic: true, material: true, materialText: 'a pinch of fine sand' },
  duration: {
    kind: 'timed',
    amount: 1,
    unit: 'minute',
    upTo: false,
    concentration: false,
    raw: '1 minute',
  },
  description: ['This spell sends creatures into a magical slumber.'],
  higherLevel: [],
} as SpellDetailsVm;

describe('openSpellDetails (the "?" in the session, E8-02)', () => {
  const data = { namePt: 'Sono', load: () => Promise.resolve(SLEEP) };

  function phone(matches: boolean) {
    // Enough of a MediaQueryList for the CDK's breakpoint observer, which the dialogs use.
    vi.stubGlobal('matchMedia', () => ({
      matches,
      addListener: () => undefined,
      removeListener: () => undefined,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
    }));
  }

  afterEach(() => vi.unstubAllGlobals());

  it('is a bottom sheet on a phone', () => {
    phone(true);
    const dialog = TestBed.inject(MatDialog);
    const sheet = TestBed.inject(MatBottomSheet);
    const openSheet = vi.spyOn(sheet, 'open');
    const openDialog = vi.spyOn(dialog, 'open');
    openSpellDetails(dialog, sheet, data);
    expect(openSheet).toHaveBeenCalledWith(
      SpellDetails,
      expect.objectContaining({ data, ariaLabel: 'Descrição de Sono' }),
    );
    expect(openDialog).not.toHaveBeenCalled();
    sheet.dismiss();
  });

  it('is a 560px dialog from a tablet up', () => {
    phone(false);
    const dialog = TestBed.inject(MatDialog);
    const sheet = TestBed.inject(MatBottomSheet);
    const openSheet = vi.spyOn(sheet, 'open');
    const openDialog = vi.spyOn(dialog, 'open');
    openSpellDetails(dialog, sheet, data);
    expect(openDialog).toHaveBeenCalledWith(
      SpellDetails,
      expect.objectContaining({ width: '560px' }),
    );
    expect(openSheet).not.toHaveBeenCalled();
    dialog.closeAll();
  });

  it('over another sheet, a phone gets a dialog drawn like a sheet, so the sheet under it stays open', () => {
    phone(true);
    const dialog = TestBed.inject(MatDialog);
    const sheet = TestBed.inject(MatBottomSheet);
    const openSheet = vi.spyOn(sheet, 'open');
    const openDialog = vi.spyOn(dialog, 'open');
    openSpellDetails(dialog, sheet, data, true);
    expect(openSheet).not.toHaveBeenCalled();
    expect(openDialog).toHaveBeenCalledWith(
      SpellDetails,
      expect.objectContaining({
        position: { bottom: '0' },
        panelClass: 'mr-sheet-over',
        data: { ...data, sheet: true },
      }),
    );
    dialog.closeAll();
  });

  it('shows Sono\'s description, the four facts and the SRD text, with "Fechar" in the footer', async () => {
    phone(false);
    const dialog = TestBed.inject(MatDialog);
    const sheet = TestBed.inject(MatBottomSheet);
    openSpellDetails(dialog, sheet, data);
    for (let i = 0; i < 20 && !document.querySelector('app-spell-details .spell__prose'); i++) {
      await new Promise((r) => setTimeout(r, 10));
    }
    const el = document.querySelector('app-spell-details')!;
    expect(el.querySelector('#spell-title')!.textContent).toBe('Sono');
    expect(el.textContent).toContain('Nome no SRD:');
    expect(el.textContent).toContain('Tempo de conjuração');
    expect(el.textContent).toContain('This spell sends creatures into a magical slumber.');
    // The title and "Fechar" stay in view while only the text scrolls.
    expect(el.querySelector('.spell__foot button')!.textContent!.trim()).toBe('Fechar');
    expect(el.querySelector('.spell__body .spell__prose')).not.toBeNull();
    dialog.closeAll();
  });
});
