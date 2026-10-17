import { TestBed } from '@angular/core/testing';

import { CharacterKind } from '../../../../gen/meurpg/characters/v1/characters_pb';
import { Role } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { SessionMessageSpeakerKind as Kind } from '../../../../gen/meurpg/play/v1/session_messages_pb';
import { CampaignsService } from '../../../core/campaigns/campaigns.service';
import { ConversationState } from '../../../core/conversation/conversation-state';
import { FakeConversationApi, line } from '../../../core/conversation/conversation-testing';
import { RosterClient } from '../../../core/maps/roster-client';
import { settle } from '../../../../test-settle';
import { ConversationPanel } from './conversation-panel';

const MEMBERS = [
  { userId: 'm', displayName: 'Mestre', role: Role.MASTER },
  { userId: 'u-p', displayName: 'Caio', role: Role.PLAYER },
  { userId: 'u-b', displayName: 'Dani', role: Role.PLAYER },
];

describe('ConversationPanel', () => {
  function setup(opts: { master?: boolean; phone?: boolean; connection?: string } = {}) {
    const api = new FakeConversationApi();
    const state = new ConversationState(
      api,
      () => 'c1',
      () => 'u-p',
    );
    TestBed.configureTestingModule({
      providers: [
        {
          provide: RosterClient,
          useValue: {
            list: () =>
              Promise.resolve([
                {
                  id: 'p',
                  name: 'Pensantus',
                  kind: CharacterKind.PLAYER,
                  playerUserId: 'u-p',
                  classSummary: 'Mago 3',
                  raceName: 'Gnomo',
                  playerName: null,
                  portraitImageId: '',
                },
                {
                  id: 'b',
                  name: 'Bartolo',
                  kind: CharacterKind.ENEMY,
                  playerUserId: '',
                  classSummary: '',
                  raceName: '',
                  playerName: null,
                  portraitImageId: '',
                },
              ]),
          },
        },
        {
          provide: CampaignsService,
          useValue: { listMembers: () => Promise.resolve({ members: MEMBERS }) },
        },
      ],
    });
    const fixture = TestBed.createComponent(ConversationPanel);
    const set = (name: string, value: unknown) => fixture.componentRef.setInput(name, value);
    set('state', state);
    set('campaignId', 'c1');
    set('isMaster', opts.master ?? false);
    set('userId', 'u-p');
    set('ownCharacterId', 'pc1');
    set('ownCharacterName', 'Pensantus');
    set('stageIds', new Set<string>());
    set('sceneName', 'A taverna do Javali Cego');
    set('connection', opts.connection ?? 'live');
    set('phone', opts.phone ?? false);
    set('active', true);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const feed = () => el.querySelector('ol') as HTMLOListElement;
    return { api, state, fixture, el, feed, set };
  }

  async function loaded(
    messages = [line('n1', { speakerKind: Kind.NARRATOR })],
    opts: { master?: boolean } = {},
  ) {
    const s = setup(opts);
    s.state.lines.set(messages);
    s.state.loaded.set(true);
    await settle(s.fixture);
    return s;
  }

  it('has the header "Conversa" and how many are at the table', async () => {
    const { el, fixture } = setup();
    await settle(fixture);
    expect(el.querySelector('h2')?.textContent?.trim()).toBe('Conversa');
    expect(el.querySelector('.panel__count')?.textContent?.trim()).toBe('2 na mesa');
  });

  it('names the feed "Mensagens da cena" and draws one line per message', async () => {
    const { feed } = await loaded([
      line('n1', { speakerKind: Kind.NARRATOR }),
      line('n2', { speakerKind: Kind.NARRATOR }),
    ]);
    expect(feed().getAttribute('aria-label')).toBe('Mensagens da cena');
    expect(feed().querySelectorAll(':scope > li')).toHaveLength(2);
  });

  it('shows the empty state once the first read found nothing', async () => {
    const { el } = await loaded([]);
    expect(el.textContent).toContain(
      'A conversa começa quando o mestre narrar a cena. Você pode escrever antes, se quiser.',
    );
  });

  it('shows no empty state before the first read', async () => {
    const { el, fixture } = setup();
    await settle(fixture);
    expect(el.textContent).not.toContain('A conversa começa');
  });

  it('says the master is writing in a status, and says nothing otherwise', async () => {
    const { el, state, fixture } = await loaded();
    expect(el.querySelector('.panel__typing')?.textContent?.trim()).toBe('');
    state.masterTyping(6);
    await settle(fixture);
    const typing = el.querySelector('.panel__typing') as HTMLElement;
    expect(typing.getAttribute('role')).toBe('status');
    expect(typing.textContent?.trim()).toBe('O mestre está escrevendo…');
    state.dispose();
  });

  it('warns while reconnecting', async () => {
    const { el, set, fixture } = setup({ connection: 'reconnecting' });
    await settle(fixture);
    const notice = el.querySelector('.mr-notice--warning') as HTMLElement;
    expect(notice.textContent).toContain('Reconectando.');
    expect(notice.textContent).toContain(
      'As mensagens que chegaram enquanto isso aparecem assim que a conexão voltar.',
    );
    set('connection', 'live');
    await settle(fixture);
    expect(el.querySelector('.mr-notice--warning')).toBeNull();
  });

  it('offers "Carregar anteriores" only when there are older lines, and loads them', async () => {
    const { el, state, api, fixture } = await loaded([line('n1', { at: 5 })]);
    expect(el.textContent).not.toContain('Carregar anteriores');
    state.hasOlder.set(true);
    await settle(fixture);
    const more = Array.from(el.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('Carregar anteriores'),
    ) as HTMLButtonElement;
    more.click();
    expect(api.listed.at(-1)).toEqual({ campaignId: 'c1', beforeId: 'n1' });
  });

  it('draws the player composer for a player and the master composer for the master', async () => {
    const player = await loaded();
    expect(player.el.querySelector('app-player-composer')).not.toBeNull();
    expect(player.el.querySelector('app-master-composer')).toBeNull();
  });

  it('draws the master composer for the master', async () => {
    const { el, fixture } = setup({ master: true });
    await settle(fixture);
    expect(el.querySelector('app-master-composer')).not.toBeNull();
    expect(el.querySelector('app-player-composer')).toBeNull();
  });

  it('shows a line that failed with "Tentar de novo", which sends it again with the same key', async () => {
    const { el, state, api, fixture } = await loaded();
    state.send({
      role: 'player',
      text: 'oi',
      as: 'out-of-character',
      characterId: '',
      onlyMaster: false,
    });
    api.failSend(new Error('down'));
    await settle(fixture, 2);
    const retry = Array.from(el.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('Tentar de novo'),
    ) as HTMLButtonElement;
    expect(retry).toBeDefined();
    retry.click();
    expect(api.sent).toHaveLength(2);
    expect(api.sent[1].idempotencyKey).toBe(api.sent[0].idempotencyKey);
  });

  it('hides "Ouvir" while the voice is off and shows it when the server says it is on', async () => {
    const message = line('n1', { speakerKind: Kind.NARRATOR, canHear: true, audioUrl: '/a' });
    const { el, state, fixture } = await loaded([message]);
    expect(el.querySelector('app-listen-button')).toBeNull();
    state.voiceEnabled.set(true);
    await settle(fixture);
    expect(el.querySelector('app-listen-button')).not.toBeNull();
  });

  it('announces a line that arrives, politely, and never the history', async () => {
    const { el, state, fixture } = await loaded([line('n1', { text: 'Velha.' })]);
    const region = el.querySelector('[aria-live="polite"][role="log"]') as HTMLElement;
    expect(region.textContent?.trim()).toBe('');
    state.receive(line('n2', { text: 'Nova narração.', at: 10 }));
    await settle(fixture);
    expect(region.textContent).toContain('Nova narração.');
    expect(region.textContent).not.toContain('Velha.');
  });

  it("does not announce the reader's own line", async () => {
    const { el, state, fixture } = await loaded([line('n1', {})]);
    state.receive(
      line('o1', {
        speakerKind: Kind.CHARACTER,
        speakerName: 'Pensantus',
        authorUserId: 'u-p',
        text: 'Eu falei.',
        at: 10,
      }),
    );
    await settle(fixture);
    expect(el.querySelector('[role="log"]')?.textContent).not.toContain('Eu falei.');
  });

  it("does not announce the master's own NPC line either", async () => {
    const { el, state, fixture } = await loaded([line('n1', {})], { master: true });
    state.receive(
      line('o2', {
        speakerKind: Kind.NPC,
        speakerName: 'Bartolo',
        authorUserId: 'u-p',
        text: 'Eu mesmo falei.',
        at: 10,
      }),
    );
    await settle(fixture);
    expect(el.querySelector('[role="log"]')?.textContent).not.toContain('Eu mesmo falei.');
  });

  it('draws the narration as a card on the phone only', async () => {
    const phone = setup({ phone: true });
    phone.state.lines.set([line('n1', { speakerKind: Kind.NARRATOR })]);
    phone.state.loaded.set(true);
    await settle(phone.fixture);
    expect(phone.el.querySelector('app-conversation-line')?.classList).toContain('line--card');
    const desktop = await loaded();
    expect(desktop.el.querySelector('app-conversation-line')?.classList).not.toContain(
      'line--card',
    );
  });

  it('scrolls to the newest line while the reader is at the bottom', async () => {
    const { feed, state, fixture } = await loaded();
    const el = feed();
    Object.defineProperty(el, 'scrollHeight', { value: 900, configurable: true });
    Object.defineProperty(el, 'clientHeight', { value: 400, configurable: true });
    el.scrollTop = 500;
    el.dispatchEvent(new Event('scroll'));
    state.receive(line('n2', { at: 10 }));
    await settle(fixture);
    expect(el.scrollTop).toBe(900);
  });

  it('leaves the reader where they are after they scrolled up', async () => {
    const { feed, state, fixture } = await loaded();
    const el = feed();
    Object.defineProperty(el, 'scrollHeight', { value: 900, configurable: true });
    Object.defineProperty(el, 'clientHeight', { value: 400, configurable: true });
    el.scrollTop = 100;
    el.dispatchEvent(new Event('scroll'));
    state.receive(line('n2', { at: 10 }));
    await settle(fixture);
    expect(el.scrollTop).toBe(100);
  });

  it('on a phone has the back arrow "Voltar à sessão" and the scene under the title', async () => {
    const { el, fixture } = setup({ phone: true });
    await settle(fixture);
    const back = el.querySelector('button[aria-label="Voltar à sessão"]') as HTMLButtonElement;
    expect(back).not.toBeNull();
    expect(el.querySelector('.panel__scene')?.textContent?.trim()).toBe('A taverna do Javali Cego');
    let went = false;
    fixture.componentInstance.back.subscribe(() => (went = true));
    back.click();
    expect(went).toBe(true);
  });

  it('on a phone says "Você é o mestre" to the master under the title', async () => {
    const { el, fixture } = setup({ phone: true, master: true });
    await settle(fixture);
    expect(el.querySelector('.panel__scene')?.textContent?.trim()).toBe('Você é o mestre');
  });

  it('on the desktop has no back arrow', async () => {
    const { el, fixture } = setup();
    await settle(fixture);
    expect(el.querySelector('button[aria-label="Voltar à sessão"]')).toBeNull();
  });
});
