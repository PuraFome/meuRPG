import { TestBed } from '@angular/core/testing';

import { CampaignRoster } from './campaign-roster';
import {
  CampaignCharacterListItemVm,
  CampaignCharactersSource,
  CampaignCharactersVm,
} from './campaign-characters.types';

const player = (id: string, state: CampaignCharacterListItemVm['state']) =>
  ({
    id,
    name: id,
    kind: 'player',
    state,
    classSummary: 'Mago 1',
    playerDisplayName: 'Ana',
  }) as CampaignCharacterListItemVm;

describe('CampaignRoster', () => {
  const listCharacters = vi.fn<(id: string) => Promise<CampaignCharactersVm>>();
  let roster: CampaignRoster;

  beforeEach(() => {
    listCharacters.mockReset().mockResolvedValue({
      playerCharacters: [player('a', 'locked'), player('b', 'pending'), player('c', 'pending')],
      npcs: [],
      hasLivingCharacter: true,
    });
    TestBed.configureTestingModule({
      providers: [
        CampaignRoster,
        { provide: CampaignCharactersSource, useValue: { listCharacters } },
      ],
    });
    roster = TestBed.inject(CampaignRoster);
  });

  it('starts loading and, once read, tells who waits for approval (the count on "Mesa")', async () => {
    expect(roster.state().status).toBe('loading');
    expect(roster.awaiting()).toEqual([]);
    await roster.ensureLoaded('camp-1');
    expect(roster.state().status).toBe('ready');
    expect(roster.awaiting().map((c) => c.id)).toEqual(['b', 'c']);
  });

  it('reads once for as many readers as ask, and again only when told to', async () => {
    await Promise.all([roster.ensureLoaded('camp-1'), roster.ensureLoaded('camp-1')]);
    await roster.ensureLoaded('camp-1');
    expect(listCharacters).toHaveBeenCalledTimes(1);
    await roster.reload('camp-1', true);
    expect(listCharacters).toHaveBeenCalledTimes(2);
  });

  it('keeps what is on screen while a quiet reload reads, and drops the answer of an older read', async () => {
    await roster.ensureLoaded('camp-1');
    const slow = roster.reload('camp-1', true);
    expect(roster.state().status).toBe('ready');
    await slow;
    expect(roster.awaiting()).toHaveLength(2);
  });

  it('says what went wrong when the list cannot be read', async () => {
    listCharacters.mockRejectedValue(new Error('down'));
    await roster.ensureLoaded('camp-1');
    expect(roster.state().status).toBe('error');
    expect(roster.awaiting()).toEqual([]);
  });
});
