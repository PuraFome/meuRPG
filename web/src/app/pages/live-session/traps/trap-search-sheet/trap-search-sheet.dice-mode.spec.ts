import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { SearchForTrapsResponseSchema } from '../../../../../gen/meurpg/play/v1/traps_pb';
import { ResourceClient } from '../../../../core/resources/resources-client';
import { TrapsClient } from '../../../../core/traps/traps-client';
import { TrapSearchSheet, type TrapSearchData } from './trap-search-sheet';

describe('TrapSearchSheet: the campaign’s dice mode (RN-18)', () => {
  function setup(diceMode: DiceMode) {
    const sent: unknown[] = [];
    const api = {
      search: async (_c: string, skill: string, die: unknown) => {
        sent.push([skill, die]);
        return create(SearchForTrapsResponseSchema, {
          roll: { diceCount: 1, diceSides: 20, faces: [14], modifier: 4, total: 18 },
          foundPointIds: [],
        });
      },
    };
    const data: TrapSearchData = {
      campaignId: 'c',
      skills: { perception: 4, investigation: 4, others: [] },
      diceMode,
      preference: DicePreference.APP,
      state: {
        refresh: async () => undefined,
        points: () => [],
      } as unknown as TrapSearchData['state'],
      inCombat: false,
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: TrapsClient, useValue: api },
        { provide: ResourceClient, useValue: { answerHeldRoll: vi.fn() } },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(TrapSearchSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const button = (name: string) =>
      Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
        b.textContent?.includes(name),
      );
    return { fixture, el, sent, button };
  }

  it('follows the campaign’s dice mode: forced app offers only "Rolar no app" and sends no typed value', async () => {
    const { fixture, el, sent, button } = setup(DiceMode.APP);
    expect(button('Digitar o resultado')).toBeUndefined();
    expect(el.querySelector('#trap-face')).toBeNull();
    button('Rolar no app')!.click();
    await fixture.whenStable();
    expect(sent).toEqual([['perception', { inApp: true }]]);
  });

  it('follows the campaign’s dice mode: forced physical offers only the typed entry and sends the face, never the total', async () => {
    const { fixture, el, sent, button } = setup(DiceMode.PHYSICAL);
    expect(button('Rolar no app')).toBeUndefined();
    button('Digitar o resultado')!.click();
    fixture.detectChanges();
    expect(button('Rolar no app')).toBeUndefined();
    const field = el.querySelector<HTMLInputElement>('#trap-face')!;
    field.value = '14';
    field.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    // +4 is the bonus: the typed 14 would read 18 as a total.
    expect(el.querySelector('.type__sum')?.textContent).toContain('14 + 4 = 18');
    button('Confirmar')!.click();
    await fixture.whenStable();
    expect(sent).toEqual([['perception', { face: 14 }]]);
  });
});
