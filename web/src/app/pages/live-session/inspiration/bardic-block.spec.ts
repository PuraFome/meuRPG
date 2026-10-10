import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import { GetOutsideInspirationResponseSchema } from '../../../../gen/meurpg/play/v1/resources_pb';
import { ResourceClient } from '../../../core/resources/resources-client';
import { BardicBlock, minutesText } from './bardic-block';

describe('BardicBlock', () => {
  function setup(state: Parameters<typeof create<typeof GetOutsideInspirationResponseSchema>>[1]) {
    const api = {
      outsideInspiration: vi
        .fn()
        .mockResolvedValue(create(GetOutsideInspirationResponseSchema, state)),
      giveBardicInspirationOutside: vi.fn().mockResolvedValue({}),
      answerOutsideInspiration: vi.fn(),
    };
    TestBed.configureTestingModule({ providers: [{ provide: ResourceClient, useValue: api }] });
    const fixture = TestBed.createComponent(BardicBlock);
    fixture.componentRef.setInput('campaignId', 'c1');
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
        b.textContent?.includes(name),
      );
    return { api, el, button, settle };
  }

  it('shows nothing to a character with no die that is not a bard', async () => {
    const { el, settle } = setup({});
    await settle();
    expect(el.textContent?.trim()).toBe('');
  });

  it('tells the holder the die, who gave it and the game time left', async () => {
    const { el, settle } = setup({
      mine: {
        characterId: 'tavo',
        characterName: 'Tavo',
        sides: 6,
        fromName: 'Orla',
        secondsLeft: 540,
      },
    });
    await settle();
    expect(el.textContent).toContain('Você tem uma Inspiração de Bardo (d6) de Orla');
    expect(el.textContent).toContain('9 min de jogo');
  });

  it('shows the bard the uses left and the party, greying who already holds a die', async () => {
    const { el, button, settle } = setup({
      isBard: true,
      sides: 8,
      usesLeft: 2,
      usesMax: 3,
      targets: [
        { characterId: 'tavo', name: 'Tavo', disabledReasonPt: '' },
        { characterId: 'nael', name: 'Nael', disabledReasonPt: 'Já tem um dado' },
      ],
    });
    await settle();
    expect(el.textContent).toContain('Usos: 2 de 3');
    expect(button('Dar a Tavo')!.disabled).toBe(false);
    expect(button('Dar a Nael')!.getAttribute('aria-disabled')).toBe('true');
    expect(el.textContent).toContain('Já tem um dado');
  });

  it('gives the die with one request and reads the state again', async () => {
    const { api, button, settle } = setup({
      isBard: true,
      sides: 8,
      usesLeft: 1,
      usesMax: 3,
      targets: [{ characterId: 'tavo', name: 'Tavo', disabledReasonPt: '' }],
    });
    await settle();
    button('Dar a Tavo')!.click();
    await settle();
    expect(api.giveBardicInspirationOutside).toHaveBeenCalledWith('c1', 'tavo', expect.any(String));
    expect(api.outsideInspiration).toHaveBeenCalledTimes(2);
  });

  it('does not offer to give in a combat, where the combat sheet does it', async () => {
    const { el, settle } = setup({
      isBard: true,
      sides: 8,
      usesLeft: 3,
      usesMax: 3,
      combatOpen: true,
    });
    await settle();
    expect(el.textContent).not.toContain('Usos:');
  });

  it('writes the minutes of game time left, rounded up', () => {
    expect(minutesText(1)).toBe('1 min');
    expect(minutesText(61)).toBe('2 min');
    expect(minutesText(600)).toBe('10 min');
  });
});
