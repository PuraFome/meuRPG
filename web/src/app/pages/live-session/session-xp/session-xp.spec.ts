import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';

import { XpMode } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  CharacterExperienceSchema,
  GetCampaignExperienceResponseSchema,
  ListTreasuresToConvertResponseSchema,
  TreasureToConvertSchema,
  XPAwardSchema,
} from '../../../../gen/meurpg/progression/v1/progression_pb';
import { RosterClient } from '../../../core/maps/roster-client';
import { ProgressionClient } from '../../../core/progression/progression-client';
import { XpChanges } from '../../../core/progression/xp-changes';
import { XpGiveButton } from '../../../shared/xp/xp-give-button';
import { SessionXp } from './session-xp';

const nbsp = ' ';

describe('SessionXp: the session\'s "Dar XP" (E9-09 state 6c)', () => {
  const experience = vi.fn();
  const listTreasures = vi.fn();
  const chest = create(TreasureToConvertSchema, {
    pointId: 'c',
    name: 'Baú de moedas',
    valuePo: 250,
  });

  async function setup() {
    TestBed.configureTestingModule({
      providers: [
        { provide: ProgressionClient, useValue: { experience, listTreasures } },
        { provide: RosterClient, useValue: { list: () => Promise.resolve([]) } },
        { provide: MatDialog, useValue: { open: vi.fn() } },
        { provide: MatBottomSheet, useValue: { open: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(SessionXp);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('campaignName', 'Mirathel');
    fixture.detectChanges();
    for (let i = 0; i < 10; i++) {
      await fixture.whenStable();
      fixture.detectChanges();
    }
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  beforeEach(() => {
    experience.mockReset().mockResolvedValue(
      create(GetCampaignExperienceResponseSchema, {
        xpMode: XpMode.GOLD,
        characters: [
          create(CharacterExperienceSchema, {
            characterId: 'p',
            name: 'Pensantus',
            level: 3,
            nextLevelXp: 2700,
          }),
        ],
      }),
    );
    listTreasures
      .mockReset()
      .mockResolvedValue(
        create(ListTreasuresToConvertResponseSchema, { treasures: [chest], total: 1 }),
      );
  });

  it('hands the button the treasures the store read, with their state, so "Voltar à cidade" never opens on a false "nenhum"', async () => {
    const { fixture } = await setup();
    const button = fixture.debugElement.query((d) => d.componentInstance instanceof XpGiveButton)
      .componentInstance as XpGiveButton;
    expect(button.treasures().map((t) => t.name)).toEqual(['Baú de moedas']);
    expect(button.treasuresTotal()).toBe(1);
    expect(button.treasuresState()).toBe('ready');
  });

  it("says what a conversion did, in the conversion's words", async () => {
    const { fixture, el } = await setup();
    const award = create(XPAwardSchema, {
      id: 'a1',
      treasureCount: 3,
      gold: 420,
      shares: [{ characterId: 'p', characterName: 'Pensantus', xp: 420 }],
    });
    fixture.debugElement
      .query((d) => d.componentInstance instanceof XpGiveButton)
      .triggerEventHandler('given', { kind: 'xp', result: { award, xpEach: 420, lostXp: 0 } });
    for (let i = 0; i < 3; i++) {
      await fixture.whenStable();
      fixture.detectChanges();
    }
    expect(el.querySelector('.status')?.textContent).toContain(
      `Voltar à cidade: Pensantus recebeu 420${nbsp}XP. Os 3${nbsp}tesouros foram convertidos.`,
    );
  });

  it('reads the XP again on `xp_changed` but leaves the treasures to the sheets (they read as they open)', async () => {
    const { fixture } = await setup();
    const calls = listTreasures.mock.calls.length;
    TestBed.inject(XpChanges).bump();
    for (let i = 0; i < 3; i++) {
      await fixture.whenStable();
      fixture.detectChanges();
    }
    expect(experience.mock.calls.length).toBeGreaterThan(1);
    expect(listTreasures.mock.calls.length).toBe(calls);
  });
});
