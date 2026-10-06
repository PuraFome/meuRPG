import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { provideRouter } from '@angular/router';

import { CharacterKind } from '../../../../gen/meurpg/characters/v1/characters_pb';
import { flat } from '../../../core/creatures/creatures-testing';
import { EncountersClient } from '../../../core/encounters/encounters-client';
import { FakeEncountersClient, GOBLIN, evaluation, line } from '../../../core/encounters/encounters-testing';
import { PartyNpcSheet, type PartyNpcData } from './party-npc-sheet';

describe('PartyNpcSheet: "Pôr um NPC no grupo" (MR-043, question 86, E10-09 state 2)', () => {
  let api: FakeEncountersClient;
  let close: ReturnType<typeof vi.fn>;

  async function setup() {
    api = new FakeEncountersClient();
    close = vi.fn();
    const before = evaluation([line(GOBLIN, 2)]);
    const data: PartyNpcData = {
      campaignId: 'camp-1',
      npcs: [
        { id: 'npc-1', name: 'Orin, o guia', kind: CharacterKind.STORY, playerUserId: '', classSummary: '', raceName: '', playerName: null, portraitImageId: '' },
        { id: 'npc-2', name: 'Velha Odra', kind: CharacterKind.STORY, playerUserId: '', classSummary: '', raceName: '', playerName: null, portraitImageId: '' },
      ],
      entries: [{ creatureKey: GOBLIN.key, count: 2 }],
      party: [],
      before,
    };
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: EncountersClient, useValue: api },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close } },
      ],
    });
    const fixture = TestBed.createComponent(PartyNpcSheet);
    const settle = async () => {
      for (let i = 0; i < 4; i++) {
        fixture.detectChanges();
        await fixture.whenStable();
      }
    };
    await settle();
    const el = fixture.nativeElement as HTMLElement;
    const button = (name: string) => Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) => (flat(b) ?? '').includes(name) || b.getAttribute('aria-label')?.includes(name))!;
    return { el, settle, button };
  }

  it('offers the campaign\'s NPCs and "Só um nome", with Orin chosen at level 3, and the budget it makes asked of the server', async () => {
    const { el } = await setup();
    expect(flat(el.querySelector('.frame__title'))).toBe('Pôr um NPC no grupo');
    expect(Array.from(el.querySelectorAll('.opt')).map((o) => flat(o))).toEqual([
      expect.stringContaining('Orin, o guia'),
      expect.stringContaining('Velha Odra'),
      expect.stringContaining('Só um nome'),
    ]);
    expect(el.querySelector<HTMLInputElement>('input[name=pn-who]:checked')?.value).toBe('npc-1');
    expect(flat(el.querySelector('output'))).toBe('3');
    // The server's budget for the party with Orin at level 3: the browser adds nothing.
    expect(api.evaluateCalls.at(-1)).toEqual({ entries: [{ creatureKey: 'monster:goblin', count: 2 }], party: [{ characterId: 'npc-1', name: '', level: 3 }] });
    expect(flat(el.querySelector('.budget__n'))).toBe('Baixa 1.400 Moderada 2.100 Alta 3.000 XP');
    expect(flat(el.querySelector('.budget__b'))).toBe('Antes: 1.250 · 1.875 · 2.600.');
    expect(flat(el.querySelector('.guide'))).toBe('Guia de dificuldade do SRD 5.2.1 (regras de 2024) · Créditos');
  });

  it('a new level asks the server again; "Pôr no grupo" closes with the NPC and its level', async () => {
    const { el, settle, button } = await setup();
    button('Mais um nível').click();
    await settle();
    expect(api.evaluateCalls.at(-1)?.party).toEqual([{ characterId: 'npc-1', name: '', level: 4 }]);
    button('Pôr no grupo').click();
    expect(close).toHaveBeenCalledWith({ characterId: 'npc-1', name: '', level: 4, label: 'Orin, o guia' });
    expect(el.textContent).not.toContain('mortal');
  });

  it('the level goes from 1 to 20', async () => {
    const { settle, button } = await setup();
    for (let i = 0; i < 25; i++) {
      button('Mais um nível').click();
    }
    await settle();
    expect(api.evaluateCalls.at(-1)?.party[0].level).toBe(20);
    for (let i = 0; i < 25; i++) {
      button('Menos um nível').click();
    }
    await settle();
    expect(api.evaluateCalls.at(-1)?.party[0].level).toBe(1);
  });

  it('"Só um nome" asks for the name in place, and sends it with no character', async () => {
    const { el, settle, button } = await setup();
    el.querySelector<HTMLInputElement>('input[name=pn-who][value=""]')!.click();
    await settle();
    button('Pôr no grupo').click();
    await settle();
    expect(flat(el.querySelector('.name__err'))).toBe('Dê um nome ao NPC do grupo.');
    expect(close).not.toHaveBeenCalled();
    const input = el.querySelector<HTMLInputElement>('input[name=label]')!;
    input.value = 'Mestre-de-armas';
    input.dispatchEvent(new Event('input'));
    await settle();
    button('Pôr no grupo').click();
    expect(close).toHaveBeenCalledWith({ characterId: '', name: 'Mestre-de-armas', level: 3, label: 'Mestre-de-armas' });
    expect(api.evaluateCalls.at(-1)?.party).toEqual([{ characterId: '', name: 'Mestre-de-armas', level: 3 }]);
  });
});
