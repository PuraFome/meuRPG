import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { MapPointKind, MapPointSchema } from '../../../../../gen/meurpg/maps/v1/maps_pb';
import { FavorBlockedReason } from '../../../../../gen/meurpg/play/v1/favor_pb';
import { SearchForTrapsResponseSchema } from '../../../../../gen/meurpg/play/v1/traps_pb';
import { FavorClient } from '../../../../core/play/favor-client';
import { FavorState } from '../../../../core/play/favor-state';
import { FakeFavorClient, favorRefused, openFavor } from '../../../../core/play/favor-testing';
import { ResourceClient } from '../../../../core/resources/resources-client';
import { TrapsClient } from '../../../../core/traps/traps-client';
import { TrapSearchSheet, type TrapSearchData } from './trap-search-sheet';

const plain = (t: string | null | undefined) => (t ?? '').replace(/\s+/g, ' ').trim();
const found = create(SearchForTrapsResponseSchema, {
  roll: { diceCount: 1, diceSides: 20, faces: [12], modifier: 4, total: 16 },
  foundPointIds: ['x'],
});

describe('TrapSearchSheet: the Inspiração', () => {
  beforeEach(() => {
    Element.prototype.scrollTo = vi.fn() as never;
  });

  function setup(holders: Record<string, string> = { pens: 'Pensantus' }, fail?: unknown) {
    TestBed.resetTestingModule();
    const sent: { skill: string; die: unknown; spend: boolean | undefined }[] = [];
    const api = {
      search: async (
        _c: string,
        skill: string,
        die: unknown,
        _key: string,
        _other?: string,
        _extra?: readonly number[],
        spend?: boolean,
      ) => {
        sent.push({ skill, die, spend });
        if (fail) {
          throw fail;
        }
        return found;
      },
    };
    const point = create(MapPointSchema, { id: 'x', kind: MapPointKind.TRAP, name: 'Fosso' });
    const data: TrapSearchData = {
      campaignId: 'c',
      skills: { perception: 4, investigation: 4, others: [], reliableTalent: [] },
      diceMode: DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
      state: {
        refresh: async () => undefined,
        points: () => [point],
      } as unknown as TrapSearchData['state'],
      inCombat: false,
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: TrapsClient, useValue: api },
        { provide: ResourceClient, useValue: { answerHeldRoll: vi.fn() } },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: vi.fn() } },
        { provide: FavorClient, useValue: new FakeFavorClient().asClient() },
      ],
    });
    openFavor(TestBed.inject(FavorState), holders);
    const fixture = TestBed.createComponent(TrapSearchSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const box = () => el.querySelector<HTMLInputElement>('app-favor-spend input[type=checkbox]');
    const roller = fixture.componentInstance as unknown as {
      rollWith(d: unknown): Promise<void>;
    };
    const roll = async () => {
      await roller.rollWith({ inApp: true });
      fixture.detectChanges();
    };
    return { fixture, el, sent, box, roll };
  }

  it('shows no box when the player has none', () => {
    expect(setup({}).box()).toBeNull();
  });

  it('sends spend_favor when ticked and nothing about it when not', async () => {
    const off = setup();
    await off.roll();
    expect(off.sent[0].spend).toBe(false);

    const on = setup();
    on.box()!.click();
    on.fixture.detectChanges();
    await on.roll();
    expect(on.sent[0].spend).toBe(true);
  });

  it('unticks the box after the spend and reads who has the Inspiração again', async () => {
    const s = setup();
    const refresh = vi.spyOn(TestBed.inject(FavorState), 'refresh').mockResolvedValue();
    s.box()!.click();
    s.fixture.detectChanges();
    await s.roll();
    expect(refresh).toHaveBeenCalledTimes(1);
  });

  it('says the refusal and unticks the box', async () => {
    const s = setup({ pens: 'Pensantus' }, favorRefused(FavorBlockedReason.NOTHING_TO_ROLL));
    s.box()!.click();
    s.fixture.detectChanges();
    await s.roll();
    expect(plain(s.el.querySelector('[role=alert]')?.textContent)).toContain(
      'Esta ação não rola um d20',
    );
    expect(s.box()!.checked).toBe(false);
  });
});
