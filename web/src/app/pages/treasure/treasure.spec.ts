import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { Code, ConnectError } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { of } from 'rxjs';

import { XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { TreasureBlockedReason, TreasureBlockedSchema, TreasureMode } from '../../../gen/meurpg/maps/v1/treasure_pb';
import { flat, isOff } from '../../core/creatures/creatures-testing';
import { type TreasureAccess, TreasureAccessCheck } from '../../core/treasure/treasure-access';
import { TreasureClient } from '../../core/treasure/treasure-client';
import { FakeTreasureClient, hoardItems, partyResponse, sampleHoard, sampleIndividual, treasureItem } from '../../core/treasure/treasure-testing';
import { TreasurePage } from './treasure';

const plain = (s: string | undefined) => s?.replace(/ /g, ' ');

describe('TreasurePage: "Gerar tesouro" (MR-044, MR-041, E10-10 states 1, 2 and 5)', () => {
  let api: FakeTreasureClient;
  let access: TreasureAccess;
  let dialogResult: unknown;
  let opened: { component: unknown; config: { data: unknown } }[];

  async function open(prep: (api: FakeTreasureClient) => void = () => undefined) {
    api = new FakeTreasureClient();
    prep(api);
    opened = [];
    TestBed.configureTestingModule({
      providers: [
        provideRouter([{ path: 'campanhas/:id/tesouro', component: TreasurePage }]),
        { provide: TreasureClient, useValue: api },
        { provide: TreasureAccessCheck, useValue: { check: async () => access } },
        {
          provide: MatDialog,
          useValue: {
            open: (component: unknown, config: { data: unknown }) => {
              opened.push({ component, config });
              return { afterClosed: () => of(dialogResult) };
            },
          },
        },
        { provide: MatBottomSheet, useValue: {} },
      ],
    });
    const harness = await RouterTestingHarness.create();
    await harness.navigateByUrl('/campanhas/camp-1/tesouro', TreasurePage);
    const settle = async () => {
      for (let i = 0; i < 4; i++) {
        harness.detectChanges();
        await harness.fixture.whenStable();
        await vi.advanceTimersByTimeAsync(0);
      }
      harness.detectChanges();
    };
    await settle();
    const el = harness.routeNativeElement as HTMLElement;
    const button = (name: string) =>
      Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) => flat(b)?.startsWith(name) || b.getAttribute('aria-label')?.startsWith(name))!;
    return { el, settle, button };
  }

  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    access = { status: 'master', campaignName: 'Mirathel', xpMode: XpMode.ENEMIES };
    dialogResult = undefined;
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('opens on the first state: the party, the mode, the level from the lowest living level, and "Nada gerado ainda."', async () => {
    const { el } = await open();
    expect(flat(el.querySelector('h1'))).toBe('Tesouro');
    expect(flat(el.querySelector('.sub'))).toBe('Mirathel · para o grupo de nível 4 e 5');
    expect(flat(el.querySelector('[data-testid=party-help]'))).toBe(
      'O grupo está nos níveis 4 e 5. A tabela usa o menor nível, para o tesouro não passar do que o grupo aguenta.',
    );
    expect(flat(el.querySelector('.step__value'))).toBe('4');
    expect(flat(el.querySelector('.empty'))).toContain('Nada gerado ainda.');
    expect(el.querySelector<HTMLInputElement>('.seg__item input:checked')?.value).toBe('hoard');
    expect(flat(el.querySelector('[data-testid=mode-hint]'))).toBe('De covil: o tesouro guardado num esconderijo.');
    expect(api.calls).toEqual(['party']);
  });

  it('with no living character the level starts at 1 and the page says to choose it', async () => {
    const { el } = await open((a) => (a.partyAnswer = partyResponse({ livingCount: 0, lowestLevel: 0, highestLevel: 0 })));
    expect(flat(el.querySelector('.sub'))).toBe('Mirathel');
    expect(flat(el.querySelector('[data-testid=party-help]'))).toContain('Escolha o nível, de 1 a 20');
    expect(flat(el.querySelector('.step__value'))).toBe('1');
  });

  it('the level is a counter between 1 and 20', async () => {
    const { el, settle, button } = await open((a) => (a.partyAnswer = partyResponse({ lowestLevel: 20, highestLevel: 20 })));
    expect(button('Mais').getAttribute('aria-disabled')).toBe('true');
    button('Menos').click();
    await settle();
    expect(flat(el.querySelector('.step__value'))).toBe('19');
  });

  it('"Gerar tesouro" sends the mode, the level and no seed, and draws what the server returns', async () => {
    const { el, settle, button } = await open();
    button('Gerar tesouro').click();
    await settle();
    expect(api.generated).toEqual([{ mode: TreasureMode.HOARD, level: 4, seed: undefined }]);
    expect(flat(el.querySelector('.result__title'))).toBe('Tesouro de covil · nível 4');
    expect(flat(el.querySelector('.result__seed'))).toBe('Semente 2209');
  });

  it('the mode and the level change the request', async () => {
    const { settle, button, el } = await open();
    el.querySelector<HTMLInputElement>('.seg__item input[value=individual]')!.click();
    button('Mais').click();
    await settle();
    button('Gerar tesouro').click();
    await settle();
    expect(api.generated).toEqual([{ mode: TreasureMode.INDIVIDUAL, level: 5, seed: undefined }]);
  });

  it('the coins show the number, what a silver piece is worth and each value in PO', async () => {
    const { el, settle, button } = await open();
    button('Gerar tesouro').click();
    await settle();
    const rows = Array.from(el.querySelectorAll('#tr-coins-h ~ .rows .row')).map((r) => plain(flat(r)));
    expect(rows).toEqual(['1.200 PP 10 PP valem 1 PO 120 PO', '340 PO 340 PO']);
    expect(plain(flat(el.querySelector('#tr-coins-h')))).toBe('Moedas 460 PO');
  });

  it('gems and art show their values, and identical gems are one row with "2 ×"', async () => {
    const { el, settle, button } = await open();
    button('Gerar tesouro').click();
    await settle();
    const gems = Array.from(el.querySelectorAll('#tr-gems-h ~ .rows .row')).map((r) => plain(flat(r)));
    expect(gems).toEqual(['2 × Ágata 6 PO cada 12 PO', 'Quartzo azul 18 PO', 'Cálice de prata gravado 27 PO']);
  });

  it('the magic items are grouped, with their English name (lang="en"), rarity, value and attunement', async () => {
    const { el, settle, button } = await open((a) => {
      const potion = hoardItems()[0]!;
      a.next = sampleHoard({ items: [potion, potion, hoardItems()[3]!, treasureItem({ key: 'item:x', name: 'Deck of Many Things', namePt: 'Baralho', valuePo: 0, rarity: 6, attunement: false })] });
    });
    button('Gerar tesouro').click();
    await settle();
    const rows = Array.from(el.querySelectorAll('.row--item'));
    expect(rows).toHaveLength(3);
    expect(plain(flat(rows[0]))).toBe('2 × Poção de Cura Potion of Healing 100 PO 50 PO cada metade de 100 PO Comum Consumível Ver descrição');
    expect(rows[0]!.querySelector('.item__en')?.getAttribute('lang')).toBe('en');
    expect(plain(flat(rows[1]))).toContain('Raro Exige sintonização');
  });

  it('the gold that becomes the point\'s gold and the items\' value are apart, and the items do not become XP', async () => {
    const { el, settle, button } = await open();
    button('Gerar tesouro').click();
    await settle();
    expect(plain(flat(el.querySelector('[data-testid=treasure-gold]')))).toBe('517 PO');
    expect(plain(flat(el.querySelector('[data-testid=treasure-items]')))).toBe('4.850 PO');
    expect(plain(flat(el.querySelector('.totals__row--items dt')))).toContain('ficam como itens, não viram XP');
    expect(plain(flat(el.querySelector('.totals__row--total')))).toBe('Valor total 5.367 PO');
    expect(plain(flat(el.querySelector('.kinds .note')))).toBe('Valores do SRD 5.2.1 (regras de 2024) · Créditos (abre em outra aba). Itens que se gastam valem a metade, menos os pergaminhos de magia.');
  });

  it('an individual treasure is coins only: no gems, no art, no items, and the total is the gold', async () => {
    const { el, settle, button } = await open((a) => (a.next = sampleIndividual()));
    el.querySelector<HTMLInputElement>('.seg__item input[value=individual]')!.click();
    await settle();
    button('Gerar tesouro').click();
    await settle();
    expect(flat(el.querySelector('.result__title'))).toBe('Tesouro individual · nível 4');
    expect(el.querySelector('#tr-gems-h')).toBeNull();
    expect(el.querySelector('#tr-items-h')).toBeNull();
    expect(plain(flat(el.querySelector('.totals__row')))).toBe('Moedas 33 PO');
    expect(el.querySelector('.totals__row--total')).toBeNull();
  });

  it('"Gerar outro" asks again without a seed and replaces the result', async () => {
    const { el, settle, button } = await open();
    button('Gerar tesouro').click();
    await settle();
    api.drawSeed = 9001n;
    button('Gerar outro').click();
    await settle();
    expect(api.generated).toHaveLength(2);
    expect(api.generated[1]!.seed).toBeUndefined();
    expect(flat(el.querySelector('.result__seed'))).toBe('Semente 9001');
  });

  it('state 5: in a campaign by enemies the line says the gold does not become XP; by gold, that it is converted in "Voltar à cidade"', async () => {
    let page = await open();
    page.button('Gerar tesouro').click();
    await page.settle();
    expect(flat(page.el.querySelector('[data-testid=treasure-gold-line]'))).toBe('Mirathel dá XP por inimigos: o ouro do tesouro não vira XP.');
    TestBed.resetTestingModule();
    access = { status: 'master', campaignName: 'Estrada de Ouro', xpMode: XpMode.GOLD };
    page = await open();
    page.button('Gerar tesouro').click();
    await page.settle();
    expect(flat(page.el.querySelector('[data-testid=treasure-gold-line]'))).toBe('Estrada de Ouro dá XP por ouro: o grupo converte isto em XP em “Voltar à cidade”.');
  });

  it('while the server rolls, the button says "Gerando..." and rests (still focusable)', async () => {
    let release = () => undefined as void;
    const { el, settle } = await open((a) => (a.gate = new Promise<void>((r) => (release = r))));
    const go = el.querySelector<HTMLButtonElement>('button.generate')!;
    go.click();
    await settle();
    expect(flat(go)).toBe('Gerando...');
    expect(isOff(go)).toBe(true);
    release();
    await settle();
    expect(isOff(el.querySelector<HTMLButtonElement>('button.generate')!)).toBe(false);
  });

  it('"Gerar outro" keeps the old result on screen with its buttons resting, and the new title takes the focus when it lands', async () => {
    const { el, settle, button } = await open();
    button('Gerar tesouro').click();
    await settle();
    let release = () => undefined as void;
    api.gate = new Promise<void>((r) => (release = r));
    api.drawSeed = 77n;
    const again = el.querySelector<HTMLButtonElement>('.act--again')!;
    again.click();
    await settle();
    expect(flat(el.querySelector('.result__seed'))).toBe('Semente 2209');
    expect(isOff(again)).toBe(true);
    expect(isOff(el.querySelector<HTMLButtonElement>('.act--go')!)).toBe(true);
    release();
    await settle();
    expect(flat(el.querySelector('.result__seed'))).toBe('Semente 77');
    expect(document.activeElement).toBe(el.querySelector('.result__title'));
  });

  it('a party that could not be read says so, with its own sentence and a way to try again, never "sem personagem vivo"', async () => {
    const { el, settle } = await open((a) => a.failWith.set('party', new ConnectError('down', Code.Unavailable)));
    expect(flat(el.querySelector('[data-testid=party-failed]'))).toContain('Não deu para ler o nível do grupo');
    expect(el.querySelector('[data-testid=party-help]')).toBeNull();
    expect(flat(el.querySelector('.options'))).not.toContain('ainda não tem personagem');
    api.failWith.clear();
    el.querySelector<HTMLButtonElement>('[data-testid=party-failed] .link')!.click();
    await settle();
    expect(el.querySelector('[data-testid=party-failed]')).toBeNull();
    expect(flat(el.querySelector('.step__value'))).toBe('4');
  });

  it('a refusal shows its words with "Tentar de novo"; no party says to choose the level', async () => {
    const { el, settle, button } = await open();
    api.failWith.set(
      'generate',
      new ConnectError('x', Code.FailedPrecondition, undefined, [{ desc: TreasureBlockedSchema, value: create(TreasureBlockedSchema, { reason: TreasureBlockedReason.NO_PARTY }) }]),
    );
    button('Gerar tesouro').click();
    await settle();
    expect(el.querySelector('[role=alert]')?.textContent).toContain('Escolha o nível do grupo');
    api.failWith.clear();
    button('Tentar de novo').click();
    await settle();
    expect(el.querySelector('.result__title')).not.toBeNull();
  });

  it('"Ver descrição" opens the item\'s sheet with its key', async () => {
    const { el, settle, button } = await open();
    button('Gerar tesouro').click();
    await settle();
    el.querySelectorAll<HTMLButtonElement>('.item__desc')[3]!.click();
    await settle();
    expect(opened).toHaveLength(1);
    expect((opened[0]!.config.data as { key: string; campaignId: string }).key).toBe('item:ring-of-protection');
    expect((opened[0]!.config.data as { campaignId: string }).campaignId).toBe('camp-1');
  });

  it('"Pôr no mapa" opens the sheet with the treasure as generated; the confirmation names the room and counts only the gold', async () => {
    const { el, settle, button } = await open();
    button('Gerar tesouro').click();
    await settle();
    dialogResult = { kind: 'placed', point: {}, mapId: 'map-1', mapName: 'Masmorra de Mirathel', place: 'Sala 3', goldPo: 517, itemCount: 4 };
    button('Pôr no mapa').click();
    await settle();
    const data = opened[0]!.config.data as { treasure: { seed: bigint; contentVersion: string }; xpMode: XpMode };
    expect(data.treasure.seed).toBe(2209n);
    expect(data.treasure.contentVersion).toBe('srd51@a8abc93b235c+fx.17');
    const done = el.querySelector('[data-testid=treasure-placed]')!;
    expect(plain(flat(done))).toContain('Tesouro posto na Sala 3. 517 PO em moedas, gemas e arte e 4 itens, escondido: só você vê.');
    // The treasure is on the map: "Abrir o mapa" is the main button and "Pôr no mapa" is gone, so a second point cannot be made by accident.
    expect(flat(el.querySelector('[data-testid=treasure-placed-line]'))).toBe('Mirathel dá XP por inimigos: o ouro do tesouro não vira XP. Os 4 itens ficam na descrição.');
    expect(Array.from(el.querySelectorAll('button')).some((b) => flat(b)?.startsWith('Pôr no mapa'))).toBe(false);
    const openLink = el.querySelector<HTMLAnchorElement>('.actions a.act--go')!;
    expect(flat(openLink)).toBe('Abrir o mapa');
    expect(openLink.getAttribute('href')).toBe('/campanhas/camp-1/mapas/map-1');
    expect(flat(el.querySelector('.act--again'))).toBe('Gerar outro');
    // A new treasure brings "Pôr no mapa" back.
    button('Gerar outro').click();
    await settle();
    expect(Array.from(el.querySelectorAll('button')).some((b) => flat(b)?.startsWith('Pôr no mapa'))).toBe(true);
    expect(el.querySelector('[data-testid=treasure-placed]')).toBeNull();
  });

  it('"again" from the sheet (the tables changed) generates a new treasure and clears the confirmation', async () => {
    const { el, settle, button } = await open();
    button('Gerar tesouro').click();
    await settle();
    dialogResult = { kind: 'again' };
    button('Pôr no mapa').click();
    await settle();
    expect(api.generated).toHaveLength(2);
    expect(el.querySelector('[data-testid=treasure-placed]')).toBeNull();
  });

  it('"Pôr no mapa" cancelled leaves the result as it was', async () => {
    const { el, settle, button } = await open();
    button('Gerar tesouro').click();
    await settle();
    button('Pôr no mapa').click();
    await settle();
    expect(api.generated).toHaveLength(1);
    expect(el.querySelector('[data-testid=treasure-placed]')).toBeNull();
    expect(isOff(button('Pôr no mapa'))).toBe(false);
  });

  it('a player is told only the master generates the treasure, and nothing is asked of the server', async () => {
    access = { status: 'forbidden' };
    const { el } = await open();
    expect(flat(el.querySelector('.mr-notice'))).toContain('Só o mestre gera o tesouro da campanha.');
    expect(api.calls).toEqual([]);
  });

  it('a campaign that is not found says so', async () => {
    access = { status: 'not-found' };
    const { el } = await open();
    expect(flat(el.querySelector('h1'))).toBe('Campanha não encontrada');
  });
});
