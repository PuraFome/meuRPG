import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { provideRouter } from '@angular/router';
import { create } from '@bufbuild/protobuf';

import { XpMode } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CombatantKind, CombatantState, EncounterStatus } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { CharacterExperienceSchema, GetCampaignExperienceResponseSchema, ListXPAwardsResponseSchema } from '../../../../../gen/meurpg/progression/v1/progression_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { RosterClient } from '../../../../core/maps/roster-client';
import { ProgressionClient } from '../../../../core/progression/progression-client';
import { CombatSummary } from './combat-summary';

const nbsp = ' ';
const combatants = [
  combatant({ id: 'p', label: 'Pensantus', kind: CombatantKind.PLAYER, characterId: 'cp' }),
  combatant({ id: 'g1', label: 'Goblin 1', defeated: true, state: CombatantState.DEFEATED, xpValue: 50 }),
  combatant({ id: 'g2', label: 'Goblin 2', defeated: true, state: CombatantState.DEFEATED, xpValue: 50 }),
];

describe('CombatSummary and the XP (E7-06)', () => {
  const experience = vi.fn();

  beforeEach(() => {
    experience.mockReset().mockResolvedValue(
      create(GetCampaignExperienceResponseSchema, {
        xpMode: XpMode.ENEMIES,
        characters: [create(CharacterExperienceSchema, { characterId: 'cp', name: 'Pensantus', level: 3, experiencePoints: 0, nextLevelXp: 300 })],
      }),
    );
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: ProgressionClient, useValue: { experience, listAwards: () => Promise.resolve(create(ListXPAwardsResponseSchema, {})), award: vi.fn() } },
        { provide: RosterClient, useValue: { list: () => Promise.resolve([]) } },
        { provide: MatDialog, useValue: { open: vi.fn() } },
        { provide: MatBottomSheet, useValue: { open: vi.fn() } },
      ],
    });
  });

  function setup(master: boolean) {
    const fixture = TestBed.createComponent(CombatSummary);
    fixture.componentRef.setInput('encounter', encounter({ id: 'enc', status: EncounterStatus.ENDED, combatants }));
    fixture.componentRef.setInput('isMaster', master);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('sessionNumber', 5);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  async function ready(fixture: ReturnType<typeof setup>['fixture']) {
    for (let i = 0; i < 3; i++) {
      await new Promise((resolve) => setTimeout(resolve, 0));
      fixture.detectChanges();
    }
  }

  const leave = (el: HTMLElement) => el.querySelector<HTMLButtonElement>('.end__leave')!;
  const filled = (b: HTMLButtonElement) => b.classList.contains('mat-mdc-unelevated-button');

  it('keeps "Voltar à sessão" outlined while the XP is still to give: the block\'s button is the one filled', async () => {
    const { fixture, el } = setup(true);
    await ready(fixture);
    expect(el.querySelector('app-combat-xp')).not.toBeNull();
    expect(filled(leave(el))).toBe(false);
    expect(leave(el).classList.contains('mat-mdc-outlined-button')).toBe(true);
    // Exactly one filled button on the screen.
    expect(Array.from(el.querySelectorAll('button')).filter(filled)).toHaveLength(1);
    expect(el.querySelector('app-combat-xp .primary')?.textContent?.trim()).toBe(`Dar 100${nbsp}XP a cada um`);
  });

  it('turns it filled once the XP is left for later', async () => {
    const { fixture, el } = setup(true);
    await ready(fixture);
    el.querySelector<HTMLButtonElement>('app-combat-xp .secondary')!.click();
    await ready(fixture);

    expect(filled(leave(el))).toBe(true);
    expect(Array.from(el.querySelectorAll('button')).filter(filled)).toHaveLength(1);
  });

  it('is filled at once when the campaign has no XP for enemies (and no block takes room)', async () => {
    experience.mockResolvedValue(create(GetCampaignExperienceResponseSchema, { xpMode: XpMode.MILESTONES, characters: [] }));
    const { fixture, el } = setup(true);
    await ready(fixture);
    expect(filled(leave(el))).toBe(true);
    expect(el.querySelector('.cols--xp')).toBeNull();
    // What the defeated were worth is not shown where it counts for nothing.
    expect(el.querySelector('.line__xp')).toBeNull();
  });

  it('shows the master what each defeated NPC is worth, and not a player', async () => {
    const master = setup(true);
    await ready(master.fixture);
    expect(Array.from(master.el.querySelectorAll('.line__xp')).map((x) => x.textContent)).toEqual([`50${nbsp}XP`, `50${nbsp}XP`]);

    TestBed.resetTestingModule();
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
    const player = setup(false);
    expect(player.el.querySelector('.line__xp')).toBeNull();
    expect(player.el.querySelector('app-combat-xp')).toBeNull();
    expect(filled(leave(player.el))).toBe(true);
    expect(experience).toHaveBeenCalledTimes(1); // only the master's block read it
  });
});
