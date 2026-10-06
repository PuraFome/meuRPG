import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { Code, ConnectError } from '@connectrpc/connect';
import { of } from 'rxjs';

import { type BestiaryAccess, BestiaryAccessCheck } from '../../../core/creatures/bestiary-access';
import { CreaturesClient } from '../../../core/creatures/creatures-client';
import { FakeCreaturesClient, flat, ogre } from '../../../core/creatures/creatures-testing';
import { BestiaryCreature } from './bestiary-creature';

describe('BestiaryCreature: the Ogre\'s stat block (MR-042, E10-08 state 3)', () => {
  let api: FakeCreaturesClient;
  let access: BestiaryAccess;
  let dialogResult: unknown;
  let opened: unknown[];

  async function open(url = '/campanhas/camp-1/bestiario/ogre?q=ogro', prep: (api: FakeCreaturesClient) => void = () => undefined) {
    api = new FakeCreaturesClient();
    api.blocks.set('monster:ogre', ogre());
    prep(api);
    opened = [];
    TestBed.configureTestingModule({
      providers: [
        provideRouter([{ path: 'campanhas/:id/bestiario/:slug', component: BestiaryCreature }]),
        { provide: CreaturesClient, useValue: api },
        { provide: BestiaryAccessCheck, useValue: { check: async () => access } },
        {
          provide: MatDialog,
          useValue: {
            open: (_c: unknown, config: unknown) => {
              opened.push(config);
              return { afterClosed: () => of(dialogResult) };
            },
          },
        },
        { provide: MatBottomSheet, useValue: {} },
      ],
    });
    const harness = await RouterTestingHarness.create();
    await harness.navigateByUrl(url, BestiaryCreature);
    const settle = async () => {
      for (let i = 0; i < 4; i++) {
        harness.detectChanges();
        await harness.fixture.whenStable();
        await vi.advanceTimersByTimeAsync(0);
      }
      harness.detectChanges();
    };
    await settle();
    return { el: harness.routeNativeElement as HTMLElement, settle };
  }

  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    access = { status: 'master', campaignName: 'Mirathel' };
    dialogResult = undefined;
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('shows the name, the SRD\'s in English, the size, the type and the alignment in Portuguese', async () => {
    const { el } = await open();
    expect(flat(el.querySelector('h1'))).toBe('Ogro');
    expect(flat(el.querySelector('.head__sub'))).toBe('Ogre · Grande · Gigante · caótico e mau');
    expect(el.querySelector('.head__sub [lang=en]')?.textContent).toBe('Ogre');
    // The back link keeps the search the master came with.
    const back = el.querySelector('.back')!;
    expect(flat(back)).toBe('Voltar ao Bestiário');
    expect(back.getAttribute('href')).toBe('/campanhas/camp-1/bestiario?q=ogro');
  });

  it('the tiles say where each number comes from: CA 11 (armadura de peles), PV 59 (7d10 + 21), 12 m (40 pés), ND 2 (450 XP)', async () => {
    const { el } = await open();
    const tiles = Array.from(el.querySelectorAll('.tile')).map((t) => flat(t));
    expect(tiles).toEqual(['CA 11 armadura de peles', 'PV 59 7d10 + 21', 'Deslocamento 12 m 40 pés', 'Nível de desafio 2 450 XP']);
    expect(Array.from(el.querySelectorAll('.ability')).map((a) => flat(a))).toContain('Força +4 valor 19');
    const lines = Array.from(el.querySelectorAll('.line')).map((l) => flat(l));
    expect(lines).toContain('Sentidos Visão no escuro 18 m, Percepção passiva 8');
    expect(lines).toContain('Idiomas Common, Giant');
    expect(lines).toContain('Bônus de proficiência +2');
    // No skills listed: no empty "Perícias —" row.
    expect(lines.some((l) => l?.startsWith('Perícias'))).toBe(false);
  });

  it('the actions are the SRD\'s text in English, marked, with the line that says so', async () => {
    const { el } = await open();
    expect(flat(el.querySelector('.srd'))).toBe('Os textos abaixo são do livro de regras (SRD 5.1), em inglês.');
    const entries = Array.from(el.querySelectorAll('.entry'));
    expect(entries.map((e) => flat(e.querySelector('h3')))).toEqual(['Greatclub', 'Javelin']);
    expect(entries.every((e) => e.getAttribute('lang') === 'en')).toBe(true);
    expect(flat(entries[0].querySelector('p'))).toContain('Melee Weapon Attack: +6 to hit');
  });

  it('credits the SRD, and has "Criar NPC" as its one button and no "Pôr no combate" yet', async () => {
    const { el } = await open();
    expect(flat(el.querySelector('.act__srd'))).toBe('Dados do SRD 5.1 (CC BY 4.0). Os alcances do texto ficam em pés, como no livro.');
    expect(el.querySelector('.act__srd a')?.getAttribute('href')).toBe('/creditos');
    expect(Array.from(el.querySelectorAll('button')).map((b) => flat(b))).toEqual(['Criar NPC']);
    expect(flat(el)).not.toContain('Pôr no combate');
  });

  it('"Criar NPC" opens the dialog with the creature; with an NPC made, the page confirms and links to its sheet', async () => {
    dialogResult = { id: 'npc-9', name: 'Capitão bandido', attacks: ['Clava grande', 'Azagaia'], existed: false };
    const { el, settle } = await open();
    el.querySelector<HTMLButtonElement>('.act__go')!.click();
    await settle();
    expect(opened).toHaveLength(1);
    expect((opened[0] as { data: { campaignId: string; creature: { summary: { key: string } } } }).data.campaignId).toBe('camp-1');
    expect((opened[0] as { data: { creature: { summary: { key: string } } } }).data.creature.summary.key).toBe('monster:ogre');
    const made = el.querySelector('.made')!;
    expect(made.getAttribute('role')).toBe('group');
    expect(flat(made.querySelectorAll('p')[0])).toBe('NPC criado: Capitão bandido. Já está na lista de NPCs.');
    // The attacks are the ones the NPC really got, from the server's answer.
    expect(flat(made.querySelectorAll('p')[1])).toBe('Ataques da ficha: Clava grande e Azagaia.');
    // One live region only: the confirmation is focused, not announced twice.
    expect(made.getAttribute('aria-live')).toBeNull();
    const links = Array.from(made.querySelectorAll('a'));
    expect(links.map((a) => flat(a))).toEqual(['Abrir a ficha', 'Voltar ao Bestiário']);
    expect(links[0].getAttribute('href')).toBe('/campanhas/camp-1/personagens/npc-9');
    expect(document.activeElement).toBe(made);
  });

  it('when the server says the NPC exists already, the page says "Já foi criado." and points to the NPC list', async () => {
    dialogResult = { id: '', name: 'Capitão bandido', attacks: [], existed: true };
    const { el, settle } = await open();
    el.querySelector<HTMLButtonElement>('.act__go')!.click();
    await settle();
    const made = el.querySelector('.made')!;
    expect(flat(made.querySelector('p'))).toBe('Já foi criado. O NPC desta tentativa já está na lista de NPCs.');
    expect(Array.from(made.querySelectorAll('a')).map((a) => flat(a))).toEqual(['Ver os NPCs', 'Voltar ao Bestiário']);
    expect(made.querySelectorAll('a')[0].getAttribute('href')).toBe('/campanhas/camp-1');
  });

  it('a dialog closed with nothing made confirms nothing', async () => {
    dialogResult = undefined;
    const { el, settle } = await open();
    el.querySelector<HTMLButtonElement>('.act__go')!.click();
    await settle();
    expect(el.querySelector('.made')?.textContent?.trim()).toBe('');
  });

  it('a creature that is not in the book says so; the stat block is not asked of a player', async () => {
    const found = await open('/campanhas/camp-1/bestiario/wyrm', (a) => a.statBlock.mockRejectedValueOnce(new ConnectError('no', Code.NotFound)));
    expect(flat(found.el.querySelector('h1'))).toBe('Criatura não encontrada');
    TestBed.resetTestingModule();
    access = { status: 'forbidden' };
    const { el } = await open();
    expect(flat(el.querySelector('.mr-notice'))).toBe('Só o mestre usa o bestiário da campanha.');
    expect(api.statBlock).not.toHaveBeenCalled();
  });

  it('a failed read says what to do, by code, and "Tentar de novo" reads again', async () => {
    const { el, settle } = await open('/campanhas/camp-1/bestiario/ogre', (a) => a.statBlock.mockRejectedValueOnce(new ConnectError('down', Code.Unavailable)));
    expect(el.querySelector('[role=alert]')?.textContent).toContain('Não deu para abrir a ficha da criatura: o servidor não respondeu. Tente de novo.');
    Array.from(el.querySelectorAll('button')).find((b) => flat(b) === 'Tentar de novo')!.click();
    await settle();
    expect(flat(el.querySelector('h1'))).toBe('Ogro');
  });
});
