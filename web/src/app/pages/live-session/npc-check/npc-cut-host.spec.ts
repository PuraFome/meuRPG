import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { Subject } from 'rxjs';
import { vi } from 'vitest';

import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { ListHeldRollsResponseSchema } from '../../../../gen/meurpg/play/v1/resources_pb';
import { NpcCheckClient } from '../../../core/npc-check/npc-check-client';
import { cuttingOffer } from '../../../core/npc-check/npc-check-testing';
import { ResourceClient } from '../../../core/resources/resources-client';
import { NpcCutHost, PASS_NOTE_MS, PASS_NOTE_TEXT } from './npc-cut-host';

function setup(offers: () => unknown[]) {
  const closing = new Subject<boolean | undefined>();
  const open = vi.fn(() => ({ afterClosed: () => closing, afterDismissed: () => closing }));
  const list = vi.fn(() =>
    Promise.resolve(create(ListHeldRollsResponseSchema, { offers: offers() as never })),
  );
  TestBed.configureTestingModule({
    providers: [
      { provide: ResourceClient, useValue: { listHeldRolls: list } },
      { provide: NpcCheckClient, useValue: {} },
      { provide: MatDialog, useValue: { open } },
      { provide: MatBottomSheet, useValue: { open } },
    ],
  });
  const fixture = TestBed.createComponent(NpcCutHost);
  fixture.componentRef.setInput('campaignId', 'c1');
  fixture.componentRef.setInput('diceMode', DiceMode.PLAYERS_CHOOSE);
  fixture.componentRef.setInput('preference', DicePreference.PHYSICAL);
  fixture.componentRef.setInput('tick', 0);
  fixture.detectChanges();
  const flush = async () => {
    for (let i = 0; i < 4; i++) {
      await fixture.whenStable();
      fixture.detectChanges();
    }
  };
  return { fixture, el: fixture.nativeElement as HTMLElement, flush, open, closing, list };
}

describe('NpcCutHost', () => {
  beforeEach(() => {
    vi.stubGlobal('matchMedia', () => ({
      matches: false,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
    }));
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it('opens the question once for a cutting-words offer, with the real die when the player rolls by hand', async () => {
    const { flush, open, fixture } = setup(() => [cuttingOffer()]);
    await flush();
    expect(open).toHaveBeenCalledTimes(1);
    const config = (
      open.mock.calls[0] as unknown as [
        unknown,
        { data: { typed: boolean }; disableClose: boolean },
      ]
    )[1];
    expect(config.data.typed).toBe(true);
    expect(config.disableClose).toBe(true);
    fixture.componentRef.setInput('tick', 1);
    await flush();
    expect(open).toHaveBeenCalledTimes(1);
  });

  it('reads again on a recheck only while a question is asked, and never for a held-rolls tick it did not get', async () => {
    const asked = setup(() => [cuttingOffer()]);
    await asked.flush();
    expect(asked.list).toHaveBeenCalledTimes(1);
    asked.fixture.componentRef.setInput('recheck', 1);
    await asked.flush();
    expect(asked.list).toHaveBeenCalledTimes(2);
  });

  it('does not read again on a recheck when no question is asked', async () => {
    const idle = setup(() => []);
    await idle.flush();
    expect(idle.list).toHaveBeenCalledTimes(1);
    idle.fixture.componentRef.setInput('recheck', 1);
    await idle.flush();
    expect(idle.list).toHaveBeenCalledTimes(1);
  });

  it('opens nothing when no offer is a question for the bard', async () => {
    const { flush, open } = setup(() => []);
    await flush();
    expect(open).not.toHaveBeenCalled();
  });

  it('after a pass, leaves one polite line for a moment and then takes it away', async () => {
    const { flush, closing, el, fixture } = setup(() => [cuttingOffer()]);
    await flush();
    vi.useFakeTimers();
    closing.next(true);
    fixture.detectChanges();
    expect(el.textContent).toContain(PASS_NOTE_TEXT);
    expect(el.querySelector('[role="status"]')).toBeTruthy();
    vi.advanceTimersByTime(PASS_NOTE_MS);
    fixture.detectChanges();
    expect(el.textContent).not.toContain(PASS_NOTE_TEXT);
  });
});
