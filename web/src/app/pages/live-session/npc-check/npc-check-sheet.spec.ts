import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';
import { vi } from 'vitest';

import { NpcCheckSource } from '../../../../gen/meurpg/play/v1/scene_pb';
import { ResourceBlockedReason, ResourceBlockedSchema } from '../../../../gen/meurpg/play/v1/resources_pb';
import { CreaturesClient } from '../../../core/creatures/creatures-client';
import { NpcCheckClient, type NpcCheckRoll } from '../../../core/npc-check/npc-check-client';
import { NpcCheckLauncher } from '../../../core/npc-check/npc-check-launcher';
import { NpcCheckState } from '../../../core/npc-check/npc-check-state';
import { candidate, npcCheckRead } from '../../../core/npc-check/npc-check-testing';
import { NpcCheckSheet, type NpcCheckSheetData } from './npc-check-sheet';

const capitao = candidate({ characterId: 'npc-cb', monsterKey: '', name: 'Capitão bandido' });
const bartolo = candidate({
  characterId: 'npc-bartolo',
  monsterKey: '',
  name: 'Bartolo',
  hasStatBlock: false,
  checks: [{ key: 'skill:persuasion', name: 'Persuasão', bonus: 0, known: false }],
});
const espiaoEscondido = candidate({
  characterId: '',
  monsterKey: 'spy',
  name: 'Espião',
  source: NpcCheckSource.STAGE,
  hidden: true,
  checks: [{ key: 'skill:stealth', name: 'Furtividade', bonus: 4, known: true }],
});
const guarda = candidate({ characterId: 'npc-g', monsterKey: '', name: 'Guarda', mapMeasures: true });

function setup(over: { picker?: () => Promise<unknown>; roll?: () => Promise<unknown>; firstId?: string } = {}) {
  const roll = vi.fn<(id: string, r: NpcCheckRoll, key: string) => Promise<unknown>>(
    over.roll ?? (() => Promise.resolve(npcCheckRead())),
  );
  const picker = vi.fn(over.picker ?? (() => Promise.resolve([capitao, bartolo, espiaoEscondido, guarda])));
  const close = vi.fn();
  const state = new NpcCheckState({ latest: () => Promise.resolve(null), skip: () => Promise.resolve(npcCheckRead()) });
  const data: NpcCheckSheetData = { campaignId: 'c1', firstId: over.firstId ?? '', state };
  TestBed.configureTestingModule({
    providers: [
      { provide: NpcCheckClient, useValue: { picker, roll } },
      { provide: CreaturesClient, useValue: {} },
      { provide: MAT_DIALOG_DATA, useValue: data },
      { provide: MatDialogRef, useValue: { close, disableClose: true } },
    ],
  });
  const fixture = TestBed.createComponent(NpcCheckSheet);
  const el = fixture.nativeElement as HTMLElement;
  const flush = async () => {
    for (let i = 0; i < 4; i++) {
      await fixture.whenStable();
      fixture.detectChanges();
    }
  };
  const text = () => (el.textContent ?? '').replace(/\s+/g, ' ');
  const choose = async (name: string) => {
    const row = Array.from(el.querySelectorAll('label.row')).find((l) => l.textContent?.includes(name));
    row?.querySelector<HTMLInputElement>('input')?.click();
    await flush();
  };
  const button = (words: string) =>
    Array.from(el.querySelectorAll('button')).find((b) => (b.textContent ?? '').includes(words));
  return { fixture, el, flush, text, choose, button, roll, picker, close, state };
}

describe('NpcCheckSheet', () => {
  it('reads who is on the stage and on the map, and keeps the roll dashed with the reason until someone is chosen', async () => {
    const { fixture, el, flush, text, button } = setup();
    fixture.detectChanges();
    expect(text()).toContain('Lendo quem está em cena e no mapa…');
    await flush();
    expect(el.querySelector('h2')?.textContent).toBe('Teste de um NPC');
    expect(text()).toContain('Fora de combate · a rolagem é só sua · um bardo do Conhecimento pode cortar');
    expect(text()).toContain('Sem bloco de estatísticas: rola com +0');
    expect(text()).toContain('Os jogadores veem');
    expect(text()).toContain('Escondido dos jogadores');
    expect(text()).toContain('Escolha quem rola e o teste.');
    expect(button('Rolar o teste')?.getAttribute('aria-disabled')).toBe('true');
  });

  it('names the roll once a creature is chosen, starting on its Deception', async () => {
    const { flush, choose, button, text } = setup();
    await flush();
    await choose('Capitão bandido');
    expect(button('Rolar Enganação do Capitão bandido')?.getAttribute('aria-disabled')).not.toBe('true');
    expect(text()).toContain('Uma perícia ou uma habilidade do Capitão bandido, com o bônus dele.');
  });

  it('opens on the NPC the stage menu named', async () => {
    const { flush, button } = setup({ firstId: 'npc-bartolo' });
    await flush();
    expect(button('Rolar Persuasão do Bartolo')).toBeTruthy();
  });

  it('shows the box ticked for a creature the players see', async () => {
    const { el, flush, choose } = setup();
    await flush();
    await choose('Capitão bandido');
    const box = el.querySelector<HTMLInputElement>('app-check-row input[type="checkbox"]');
    expect(box?.checked).toBe(true);
    expect(el.textContent).toContain('Os bardos veem a criatura e ela os ouve');
  });

  it('starts the box unticked for a creature the master hid, and says the bards would read "uma criatura"', async () => {
    const { el, flush, choose, text } = setup();
    await flush();
    await choose('Espião');
    expect(el.querySelector<HTMLInputElement>('app-check-row input[type="checkbox"]')?.checked).toBe(false);
    expect(text()).toContain('os bardos leem “uma criatura”, sem o nome');
  });

  it('shows no box when the map measures, only the quiet line', async () => {
    const { el, flush, choose } = setup();
    await flush();
    await choose('Guarda');
    expect(el.querySelector('app-check-row')).toBeNull();
    expect(el.querySelector('[data-testid="measured"]')?.textContent).toContain('o app mede os 18 m e a visão');
  });

  it('says a creature with no number for the check rolls +0, beside the check', async () => {
    const { el, flush, choose } = setup();
    await flush();
    await choose('Bartolo');
    expect(el.querySelector('[data-testid="no-bonus"]')?.textContent).toContain(
      'Sem bônus para Persuasão: o d20 conta com +0 e você soma o bônus que achar justo.',
    );
    expect(el.querySelector('option[data-side="sem bônus"]')).toBeTruthy();
  });

  it('rolls in the app, hands the check to the card and closes', async () => {
    const { flush, choose, button, roll, close, state } = setup();
    await flush();
    await choose('Capitão bandido');
    button('Rolar Enganação do Capitão bandido')?.click();
    await flush();
    expect(roll).toHaveBeenCalledTimes(1);
    expect(roll.mock.calls[0][1]).toEqual({
      characterId: 'npc-cb',
      monsterKey: '',
      checkKey: 'skill:deception',
      die: { inApp: true },
      bardSees: true,
    });
    expect(close).toHaveBeenCalledWith(true);
    expect(state.card()?.title).toBe('Capitão bandido · Enganação');
  });

  it('rolls with the typed d20 only from 1 to 20', async () => {
    const { el, flush, choose, button, roll, text } = setup();
    await flush();
    await choose('Capitão bandido');
    Array.from(el.querySelectorAll<HTMLInputElement>('app-segmented input[type="radio"]'))
      .find((i) => i.closest('label')?.textContent?.includes('Digitar o d20'))
      ?.click();
    await flush();
    expect(text()).toContain('O d20 vai de 1 a 20.');
    const field = el.querySelector<HTMLInputElement>('app-text-field input');
    const type = async (v: string) => {
      field!.value = v;
      field!.dispatchEvent(new Event('input'));
      await flush();
    };
    await type('21');
    expect(text()).toContain('Digite um número de 1 a 20.');
    expect(button('Rolar Enganação do Capitão bandido')?.getAttribute('aria-disabled')).toBe('true');
    await type('11');
    button('Rolar Enganação do Capitão bandido')?.click();
    await flush();
    expect(roll.mock.calls[0][1].die).toEqual({ face: 11 });
  });

  it('says a combat began, rolls nothing and tells the entry', async () => {
    const blocked = new ConnectError('x', Code.FailedPrecondition, undefined, [
      { desc: ResourceBlockedSchema, value: create(ResourceBlockedSchema, { reason: ResourceBlockedReason.COMBAT_OPEN }) },
    ]);
    const { el, flush, choose, button, close, state } = setup({ roll: () => Promise.reject(blocked) });
    await flush();
    await choose('Capitão bandido');
    button('Rolar Enganação')?.click();
    await flush();
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('Um combate começou.');
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('não foi rolado');
    expect(TestBed.inject(NpcCheckLauncher).combatOpen()).toBe(true);
    expect(close).not.toHaveBeenCalled();
    expect(state.card()).toBeNull();
  });

  it('says it could not read the creatures, and reads again', async () => {
    const picker = vi.fn().mockRejectedValueOnce(new Error('x')).mockResolvedValueOnce([capitao]);
    const { el, flush, button, text } = setup({ picker });
    await flush();
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('Não deu para ler as criaturas.');
    button('Tentar de novo')?.click();
    await flush();
    expect(text()).toContain('Capitão bandido');
    expect(picker).toHaveBeenCalledTimes(2);
  });
});
