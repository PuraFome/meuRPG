import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { of } from 'rxjs';
import { provideRouter } from '@angular/router';

import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import type { VitalsVm } from '../live-session.types';
import type { HitDiceSheetData } from '../hit-dice-sheet/hit-dice-sheet';
import { pensantusVitals } from '../testing';
import { PlayerVitals } from './player-vitals';

describe('PlayerVitals as a beast (MR-037, E9-11)', () => {
  function render(over: Parameters<typeof pensantusVitals>[0], beastAc: number | null) {
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    const fixture = TestBed.createComponent(PlayerVitals);
    fixture.componentRef.setInput(
      'vitals',
      pensantusVitals({ name: 'Sálvia', hitPointsCurrent: 38, hitPointsMax: 38, ...over }),
    );
    fixture.componentRef.setInput('sheet', { armorClass: 12, summary: 'Druida 5', senses: [] });
    fixture.componentRef.setInput('campaignId', 'camp');
    fixture.componentRef.setInput('beastAc', beastAc);
    fixture.componentRef.setInput('compact', true);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }
  const flat = (n: Element | null) => n?.textContent?.replace(/\s+/g, ' ').trim();

  it('shows the two reserves where the hit points box is, the beast\'s armor class, and "Sem magias" in place of the slots', () => {
    const el = render(
      {
        wildShape: {
          beastKey: 'monster:wolf',
          beastNamePt: 'Lobo',
          hitPointsCurrent: 11,
          hitPointsMax: 11,
        },
      },
      13,
    );
    expect(Array.from(el.querySelectorAll('app-wild-pools .pool'), flat)).toEqual([
      'PV do Lobo11 de 11',
      'PV da Sálvia38 de 38',
    ]);
    expect(el.querySelector('.hp')).toBeNull();
    // One armor class on the page, the beast's: not the druid's own 12.
    expect(flat(el.querySelector('.shield'))).toContain('13');
    expect(flat(el.querySelector('.shield'))).not.toContain('12');
    expect(flat(el.querySelector('.shield .stats__label'))).toBe('CA do Lobo');
    expect(el.querySelector('.slots')).toBeNull();
    expect(flat(el.querySelector('.nospells'))).toBe('blockSem magias na forma de fera.');
  });

  it('is the ordinary card otherwise', () => {
    const el = render({}, null);
    expect(el.querySelector('app-wild-pools')).toBeNull();
    expect(flat(el.querySelector('.shield'))).toContain('12');
    expect(el.querySelector('.slots')).not.toBeNull();
  });
});

describe('PlayerVitals hit dice by size (decisions batch 2 B, 6)', () => {
  function render(over: Parameters<typeof pensantusVitals>[0]) {
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    const fixture = TestBed.createComponent(PlayerVitals);
    fixture.componentRef.setInput('vitals', pensantusVitals(over));
    fixture.componentRef.setInput('campaignId', 'camp');
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }
  const flat = (n: Element | null) => n?.textContent?.replace(/\s+/g, ' ').trim();

  it('says the dice by size and what is left of each', () => {
    const el = render({
      hitDice: '5d10 e 1d6',
      hitDiceSizes: [
        { faces: 10, total: 5, used: 2 },
        { faces: 6, total: 1, used: 0 },
      ],
      hitDiceTotal: 6,
      hitDiceUsed: 2,
    });
    const box = Array.from(el.querySelectorAll('.box')).find((b) =>
      flat(b)?.startsWith('Dados de vida'),
    );
    expect(flat(box?.querySelector('.box__value') ?? null)).toBe('5d10 e 1d6');
    expect(flat(box?.querySelector('.box__words') ?? null)).toBe('Restam 3 de 5d10 e 1 de 1d6');
  });

  it('says one size as before: what is left of how many', () => {
    const el = render({});
    expect(flat(el.querySelector('.box__words'))).toBe('Restam 2 de 3d6');
  });
});

describe('PlayerVitals "Gastar dados de vida" (decisions batch 2 B 6)', () => {
  const dialogOpen = vi.fn();
  const sheetOpen = vi.fn();

  beforeEach(() => {
    dialogOpen.mockReset().mockReturnValue({ afterClosed: () => of(true) });
    sheetOpen.mockReset().mockReturnValue({ afterDismissed: () => of(true) });
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: MatDialog, useValue: { open: dialogOpen } },
        { provide: MatBottomSheet, useValue: { open: sheetOpen } },
      ],
    });
  });

  function render(over: Parameters<typeof pensantusVitals>[0] = {}, compact = false) {
    const fixture = TestBed.createComponent(PlayerVitals);
    fixture.componentRef.setInput('vitals', pensantusVitals(over));
    fixture.componentRef.setInput('campaignId', 'camp');
    fixture.componentRef.setInput('compact', compact);
    fixture.componentRef.setInput('diceMode', DiceMode.PHYSICAL);
    fixture.componentRef.setInput('dicePreference', DicePreference.PHYSICAL);
    const changes: VitalsVm[] = [];
    fixture.componentInstance.vitalsChange.subscribe((v) => changes.push(v));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const button = () => el.querySelector<HTMLButtonElement>('.js-hit-dice')!;
    return { fixture, el, button, changes };
  }

  /** What the page handed the sheet, from whichever container opened it. */
  const opened = () =>
    (dialogOpen.mock.calls[0] ?? sheetOpen.mock.calls[0])[1].data as HitDiceSheetData;

  it('offers the button on the card, with dice left', () => {
    const { button } = render();
    expect(button().textContent).toContain('Gastar dados de vida');
    expect(button().disabled).toBe(false);
    expect(button().classList.contains('mr-button--off')).toBe(false);
  });

  it("is not on the combat's compact card: a rest does not happen in a combat", () => {
    const { button } = render({}, true);
    expect(button()).toBeNull();
  });

  it('opens the sheet with the campaign, the dice setting and the live vitals', () => {
    const { button, fixture } = render();
    button().click();
    expect(dialogOpen.mock.calls.length + sheetOpen.mock.calls.length).toBe(1);
    const data = opened();
    expect(data.campaignId).toBe('camp');
    expect(data.diceMode).toBe(DiceMode.PHYSICAL);
    expect(data.preference).toBe(DicePreference.PHYSICAL);
    expect(data.vitals().characterId).toBe('pensantus');
    // The same signal as the card's: what the stream brings shows in the sheet.
    fixture.componentRef.setInput('vitals', pensantusVitals({ hitPointsCurrent: 3, revision: 9 }));
    expect(data.vitals().hitPointsCurrent).toBe(3);
  });

  it('hands the vitals of each die to the page', () => {
    const { button, changes } = render();
    button().click();
    const next = pensantusVitals({ revision: 2 });
    opened().apply(next);
    expect(changes).toEqual([next]);
  });

  it('is dashed and says why when every die is used, and opens nothing', () => {
    const { el, button } = render({
      hitDiceSizes: [{ faces: 6, total: 3, used: 3 }],
      hitDiceUsed: 3,
    });
    expect(button().classList.contains('mr-button--off')).toBe(true);
    expect(button().getAttribute('aria-describedby')).toBe('dice-reason');
    expect(el.querySelector('#dice-reason')?.textContent).toContain(
      'Não há dados de vida para gastar.',
    );
    button().click();
    expect(dialogOpen).not.toHaveBeenCalled();
    expect(sheetOpen).not.toHaveBeenCalled();
  });
});
