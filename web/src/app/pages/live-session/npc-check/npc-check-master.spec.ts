import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { vi } from 'vitest';

import { NpcCheckClient } from '../../../core/npc-check/npc-check-client';
import { npcCheckRead } from '../../../core/npc-check/npc-check-testing';
import { NpcCheckMaster } from './npc-check-master';

function setup(settled: boolean) {
  const latest = vi.fn(() =>
    Promise.resolve(npcCheckRead({ settled, total: 13, asks: settled ? [] : undefined })),
  );
  TestBed.configureTestingModule({
    providers: [
      { provide: NpcCheckClient, useValue: { latest } },
      { provide: MatDialog, useValue: { open: vi.fn() } },
      { provide: MatBottomSheet, useValue: { open: vi.fn() } },
    ],
  });
  const fixture = TestBed.createComponent(NpcCheckMaster);
  fixture.componentRef.setInput('campaignId', 'c1');
  fixture.detectChanges();
  const flush = async () => {
    for (let i = 0; i < 4; i++) {
      await fixture.whenStable();
      fixture.detectChanges();
    }
  };
  return { fixture, flush, latest };
}

describe('NpcCheckMaster', () => {
  it('reads the check again on a recheck only while the check waits', async () => {
    const waiting = setup(false);
    await waiting.flush();
    expect(waiting.latest).toHaveBeenCalledTimes(1);
    waiting.fixture.componentRef.setInput('recheck', 1);
    await waiting.flush();
    expect(waiting.latest).toHaveBeenCalledTimes(2);
  });

  it('does not read again on a recheck once the check settled', async () => {
    TestBed.resetTestingModule();
    const settled = setup(true);
    await settled.flush();
    expect(settled.latest).toHaveBeenCalledTimes(1);
    settled.fixture.componentRef.setInput('recheck', 1);
    await settled.flush();
    expect(settled.latest).toHaveBeenCalledTimes(1);
  });
});
