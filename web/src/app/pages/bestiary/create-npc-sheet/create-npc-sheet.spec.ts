import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { Code, ConnectError } from '@connectrpc/connect';

import { CreaturesClient } from '../../../core/creatures/creatures-client';
import {
  FakeCreaturesClient,
  flat,
  invalidField,
  isOff,
  ogre,
  summary,
} from '../../../core/creatures/creatures-testing';
import { CreateNpcSheet, type CreateNpcData } from './create-npc-sheet';

describe('CreateNpcSheet: "Criar NPC" from a creature (MR-042, RN-29, E10-08 state 8)', () => {
  let api: FakeCreaturesClient;
  let close: ReturnType<typeof vi.fn>;

  async function setup(creature = ogre()) {
    api = new FakeCreaturesClient();
    close = vi.fn();
    const data: CreateNpcData = { campaignId: 'camp-1', creature };
    TestBed.configureTestingModule({
      providers: [
        { provide: CreaturesClient, useValue: api },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close } },
      ],
    });
    const fixture = TestBed.createComponent(CreateNpcSheet);
    const settle = async () => {
      for (let i = 0; i < 3; i++) {
        fixture.detectChanges();
        await fixture.whenStable();
      }
    };
    await settle();
    const el = fixture.nativeElement as HTMLElement;
    const button = (name: string) =>
      Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
        flat(b)?.includes(name),
      )!;
    const type = (text: string) => {
      const input = el.querySelector<HTMLInputElement>('input[name=name]')!;
      input.value = text;
      input.dispatchEvent(new Event('input'));
    };
    const role = (label: string) => {
      const radio = Array.from(el.querySelectorAll<HTMLLabelElement>('.seg__opt'))
        .find((l) => flat(l) === label)!
        .querySelector('input')!;
      radio.click();
    };
    return { fixture, el, button, settle, type, role };
  }

  it('opens with the creature in "Baseado em", its Portuguese name for the NPC, and Minion chosen', async () => {
    const { el } = await setup();
    expect(flat(el.querySelector('.frame__title'))).toBe('Criar NPC');
    expect(flat(el.querySelector('.frame__sub'))).toBe('Do bestiário: Ogro');
    expect(flat(el.querySelector('.based'))).toBe('Baseado em Ogro Ogre · ND 2');
    expect(el.querySelector<HTMLInputElement>('input[name=name]')!.value).toBe('Ogro');
    expect(flat(el.querySelector('.field__count'))).toBe('4 de 80');
    expect(el.querySelector('.based [lang=en]')?.textContent).toBe('Ogre');
  });

  it('offers only Minion and NPC de história for the role: Inimigo and Boss need a full sheet', async () => {
    const { el } = await setup();
    expect(Array.from(el.querySelectorAll('.seg__opt')).map((l) => flat(l))).toEqual([
      'Minion',
      'NPC de história',
    ]);
    expect(el.querySelector<HTMLInputElement>('.seg__opt input:checked')?.value).toBe('minion');
    expect(flat(el.querySelector('.role__hint'))).toContain('Um capanga');
  });

  it('shows the numbers that come from the creature: CA, PV, the speed in metres and the six abilities in full words', async () => {
    const { el } = await setup();
    const nums = Array.from(el.querySelectorAll('.num')).map((n) => flat(n));
    expect(nums).toEqual([
      'CA 11',
      'PV 59',
      'Deslocamento 12 m',
      'Força 19',
      'Destreza 8',
      'Constituição 16',
      'Inteligência 5',
      'Sabedoria 7',
      'Carisma 7',
    ]);
    // The narrowest phones read them as one line instead.
    expect(flat(el.querySelector('.nums__line'))).toBe(
      'CA 11 · PV 59 · 12 m e as habilidades vêm da criatura.',
    );
  });

  it("says the attacks that come along and that the bestiary's creature does not change", async () => {
    const { el } = await setup();
    expect(flat(el.querySelector('.note'))).toBe(
      'Faz uma ficha básica de NPC, com os ataques da criatura (Clava grande e Azagaia), que você renomeia e edita depois. A criatura do bestiário não muda.',
    );
  });

  it("creates a Minion with the master's name and closes with the NPC", async () => {
    const { button, type, settle } = await setup();
    type('Capitão bandido');
    await settle();
    button('Criar NPC').click();
    await settle();
    expect(api.npcCalls).toEqual([
      {
        creatureKey: 'monster:ogre',
        name: 'Capitão bandido',
        role: 'minion',
        key: expect.stringMatching(/^[0-9a-f-]{36}$/),
      },
    ]);
    expect(close).toHaveBeenCalledWith({
      id: 'npc-1',
      name: 'Capitão bandido',
      attacks: ['Clava grande', 'Azagaia'],
      existed: false,
    });
  });

  it('creates an NPC de história when that role is chosen, trimming the name', async () => {
    const { button, type, role, settle } = await setup();
    type('  Grak, o ogro  ');
    role('NPC de história');
    await settle();
    button('Criar NPC').click();
    await settle();
    expect(api.npcCalls[0]).toMatchObject({ name: 'Grak, o ogro', role: 'story' });
    expect(close).toHaveBeenCalledTimes(1);
  });

  it('a double tap makes one NPC: the second tap waits for the first', async () => {
    const { button, settle } = await setup();
    const go = button('Criar NPC');
    go.click();
    go.click();
    await settle();
    expect(api.npcCalls).toHaveLength(1);
  });

  it('a retry after a failure sends the same key, so a lost answer cannot make two NPCs', async () => {
    const { button, el, settle } = await setup();
    api.npcFailures = [new ConnectError('down', Code.Unavailable)];
    button('Criar NPC').click();
    await settle();
    expect(el.querySelector('[role=alert]')?.textContent).toContain(
      'Não deu para criar o NPC: o servidor não respondeu. Tente de novo.',
    );
    expect(close).not.toHaveBeenCalled();
    expect(isOff(button('Criar NPC'))).toBe(false);
    button('Criar NPC').click();
    await settle();
    expect(api.npcCalls).toHaveLength(2);
    expect(api.npcCalls[1].key).toBe(api.npcCalls[0].key);
    expect(close).toHaveBeenCalledTimes(1);
  });

  it('the key stays the same through edits of the name and the role: one key per open dialog', async () => {
    const { button, type, role, settle } = await setup();
    api.npcFailures = [
      new ConnectError('down', Code.Unavailable),
      new ConnectError('down', Code.Unavailable),
    ];
    button('Criar NPC').click();
    await settle();
    type('Vesna, a capitã');
    await settle();
    button('Criar NPC').click();
    await settle();
    role('NPC de história');
    await settle();
    button('Criar NPC').click();
    await settle();
    expect(new Set(api.npcCalls.map((c) => c.key)).size).toBe(1);
    expect(api.npcCalls.map((c) => c.name)).toEqual(['Ogro', 'Vesna, a capitã', 'Vesna, a capitã']);
  });

  it('a key the server already used means the first try made the NPC: the dialog closes saying so, with no error', async () => {
    const { el, button, settle } = await setup();
    api.npcFailures = [invalidField('idempotency_key')];
    button('Criar NPC').click();
    await settle();
    expect(close).toHaveBeenCalledWith({ id: '', name: 'Ogro', attacks: [], existed: true });
    expect(el.querySelector('[role=alert]')).toBeNull();
  });

  it('an empty name asks for one in place, with the focus back on the field, and sends nothing', async () => {
    const { el, button, type, settle } = await setup();
    type('   ');
    await settle();
    button('Criar NPC').click();
    await settle();
    expect(flat(el.querySelector('.field__err'))).toBe('Dê um nome ao NPC.');
    expect(document.activeElement).toBe(el.querySelector('input[name=name]'));
    expect(el.querySelector('input[name=name]')?.getAttribute('aria-invalid')).toBe('true');
    expect(api.npcCalls).toEqual([]);
    type('Vesna');
    await settle();
    expect(el.querySelector('.field__err')).toBeNull();
  });

  it.each([
    [
      new ConnectError('refused', Code.PermissionDenied),
      'Só o mestre da campanha usa o bestiário e faz NPCs.',
    ],
    [
      new ConnectError('full', Code.ResourceExhausted),
      'A campanha chegou ao limite de 1.000 personagens e NPCs. Apague um para criar outro.',
    ],
    [
      invalidField('name'),
      'Não deu para criar o NPC: o nome precisa ter de 1 a 80 letras, numa linha só. Confira e tente de novo.',
    ],
    [
      invalidField('creature_key'),
      'Essa criatura não está no bestiário. Volte à lista e escolha outra.',
    ],
    [invalidField('kind'), 'Escolha Minion ou NPC de história.'],
    [
      new ConnectError('refused', Code.NotFound),
      'Essa campanha não existe, ou você não é mais membro dela. Volte para Minhas campanhas.',
    ],
  ])('a refusal is said in Portuguese by its code and its field (%#)', async (error, message) => {
    const { el, button, settle } = await setup();
    api.npcFailures = [error];
    button('Criar NPC').click();
    await settle();
    expect(flat(el.querySelector('[role=alert]'))).toBe(message);
    expect(close).not.toHaveBeenCalled();
  });

  it('"Cancelar" and the close button close it with nothing made', async () => {
    const { el, button } = await setup();
    button('Cancelar').click();
    expect(close.mock.calls[0][0]).toBeUndefined();
    el.querySelector<HTMLButtonElement>('.frame__close')!.click();
    expect(close).toHaveBeenCalledTimes(2);
    expect(api.npcCalls).toEqual([]);
  });

  it('a creature with no attack a basic sheet can hold says its attacks stay in the stat block', async () => {
    const { el } = await setup(
      ogre({
        actions: [],
        npcAttackNames: [],
        summary: summary('monster:ghost', 'Fantasma', { name: 'Ghost' }),
      }),
    );
    expect(flat(el.querySelector('.note'))).toContain(
      'os ataques dela ficam na ficha do bestiário',
    );
  });
});
