import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { MapPointKind, MapPointSchema } from '../../../../../gen/meurpg/maps/v1/maps_pb';
import { OutsideInspirationOfferSchema } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  ResourceBlockedReason,
  ResourceBlockedSchema,
} from '../../../../../gen/meurpg/play/v1/resources_pb';
import { SearchForTrapsResponseSchema } from '../../../../../gen/meurpg/play/v1/traps_pb';
import { LUCKY_TRAIT } from '../../../../core/combat/lucky';
import { luckyOffer } from '../../../../core/play/scene-testing';
import { ResourceClient } from '../../../../core/resources/resources-client';
import { TrapsClient } from '../../../../core/traps/traps-client';
import { TrapSearchSheet, type TrapSearchData } from './trap-search-sheet';

const flat = (n: Element | null | undefined) => n?.textContent?.replace(/\s+/g, ' ').trim() ?? '';

/** The search's d20 after Lucky rolled it again: 1 → `to`, with Investigação's +4. */
const rerolled = (to: number) => ({
  diceCount: 1,
  diceSides: 20,
  faces: [to],
  modifier: 4,
  total: to + 4,
  rerolled: [{ index: 0, from: 1, to, sourceKey: LUCKY_TRAIT }],
});

describe('TrapSearchSheet: the halfling’s Lucky question', () => {
  const answerOutsideLucky = vi.fn();
  const answerOutsideInspiration = vi.fn();

  beforeEach(() => {
    Element.prototype.scrollTo = vi.fn() as never;
    answerOutsideLucky.mockReset();
    answerOutsideInspiration.mockReset();
  });

  function setup(responses: unknown[]) {
    const api = {
      search: async () => {
        const next = responses.shift();
        if (next instanceof Error) {
          throw next;
        }
        return next;
      },
    };
    const data: TrapSearchData = {
      campaignId: 'c',
      skills: { perception: 4, investigation: 4, others: [] },
      diceMode: DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
      state: {
        refresh: async () => undefined,
        points: () => [
          create(MapPointSchema, { id: 'x', kind: MapPointKind.TRAP, name: 'Fosso escondido' }),
        ],
      } as unknown as TrapSearchData['state'],
      inCombat: false,
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: TrapsClient, useValue: api },
        {
          provide: ResourceClient,
          useValue: { answerOutsideLucky, answerOutsideInspiration },
        },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(TrapSearchSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const settle = async () => {
      for (let i = 0; i < 3; i++) {
        await fixture.whenStable();
        fixture.detectChanges();
      }
    };
    const button = (name: string) =>
      Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
        flat(b).includes(name),
      );
    return { el, settle, button };
  }

  it('asks after a d20 of 1 and tells nothing of the search until it is answered', async () => {
    const { el, settle, button } = setup([
      create(SearchForTrapsResponseSchema, { luckyOffer: luckyOffer([1], 4) }),
    ]);
    button('Rolar no app')!.click();
    await settle();
    const text = flat(el);
    expect(text).toContain('Tirou 1: rolar de novo?');
    expect(text).not.toMatch(/Achou|Nada/);
    expect(button('Ficar com o 1')).toBeTruthy();
    expect(button('Fechar')).toBeTruthy();
  });

  it('"Rolar de novo" makes the search with the new d20 and says "Sortudo: 1 → 14"', async () => {
    const { el, settle, button } = setup([
      create(SearchForTrapsResponseSchema, { luckyOffer: luckyOffer([1], 4) }),
    ]);
    answerOutsideLucky.mockResolvedValue({
      result: {
        case: 'trapSearch',
        value: create(SearchForTrapsResponseSchema, {
          roll: rerolled(14),
          foundPointIds: ['x'],
        }),
      },
    });
    button('Rolar no app')!.click();
    await settle();
    button('Rolar de novo')!.click();
    await settle();
    expect(answerOutsideLucky).toHaveBeenCalledWith(
      'c',
      'h1',
      true,
      { inApp: true },
      expect.any(String),
    );
    const text = flat(el);
    expect(text).toContain('Achou');
    expect(text).toContain('Sortudo: 1 → 14');
    expect(text).not.toContain('Tirou 1: rolar de novo?');
  });

  it('after the new d20, a Bardic Inspiration die the character holds is asked about, on the new roll', async () => {
    const { el, settle, button } = setup([
      create(SearchForTrapsResponseSchema, { luckyOffer: luckyOffer([1], 4) }),
    ]);
    answerOutsideLucky.mockResolvedValue({
      result: {
        case: 'trapSearch',
        value: create(SearchForTrapsResponseSchema, {
          inspirationOffer: create(OutsideInspirationOfferSchema, {
            holdId: 'h1',
            sides: 8,
            fromName: 'Orla',
            d20: rerolled(7),
          }),
        }),
      },
    });
    button('Rolar no app')!.click();
    await settle();
    button('Rolar de novo')!.click();
    await settle();
    const text = flat(el);
    expect(text).toContain('Usar a Inspiração de Bardo (d8)?');
    expect(text).toContain('Sortudo: 1 → 7');
    expect(text).not.toMatch(/Achou|Nada/);
  });

  it('keeps the 1 when the player says so', async () => {
    const { settle, button } = setup([
      create(SearchForTrapsResponseSchema, { luckyOffer: luckyOffer([1], 4) }),
    ]);
    answerOutsideLucky.mockResolvedValue({
      result: {
        case: 'trapSearch',
        value: create(SearchForTrapsResponseSchema, {
          roll: { diceCount: 1, diceSides: 20, faces: [1], modifier: 4, total: 5 },
        }),
      },
    });
    button('Rolar no app')!.click();
    await settle();
    button('Ficar com o 1')!.click();
    await settle();
    expect(answerOutsideLucky).toHaveBeenCalledWith('c', 'h1', false, null, expect.any(String));
  });

  it('says another roll waits for the Lucky answer when the server refuses with LUCKY_PENDING', async () => {
    const { el, settle, button } = setup([
      new ConnectError('pending', Code.FailedPrecondition, undefined, [
        {
          desc: ResourceBlockedSchema,
          value: create(ResourceBlockedSchema, { reason: ResourceBlockedReason.LUCKY_PENDING }),
        },
      ]),
    ]);
    button('Rolar no app')!.click();
    await settle();
    expect(flat(el.querySelector('[role=alert]'))).toContain(
      'Responda primeiro à pergunta do Sortudo',
    );
  });
});
