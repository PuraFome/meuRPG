import { TestBed } from '@angular/core/testing';

import { SessionMessageSpeakerKind as Kind } from '../../../../gen/meurpg/play/v1/session_messages_pb';
import { line } from '../../../core/conversation/conversation-testing';
import { ListenController } from '../../../core/conversation/listen-controller';
import { type PlayerDirectory, viewLine } from '../../../core/conversation/message-view';
import type { SessionMessage } from '../../../../gen/meurpg/play/v1/session_messages_pb';
import { ConversationLine } from './conversation-line';

const DIRECTORY: PlayerDirectory = new Map([
  ['u-p', 'Pensantus'],
  ['u-b', 'Brisa'],
]);

describe('ConversationLine', () => {
  function render(
    message: SessionMessage,
    opts: { master?: boolean; voice?: boolean; card?: boolean } = {},
  ) {
    const fixture = TestBed.createComponent(ConversationLine);
    const view = viewLine(message, { userId: 'u-p', isMaster: opts.master ?? false }, DIRECTORY);
    fixture.componentRef.setInput('view', view);
    fixture.componentRef.setInput('listen', new ListenController());
    fixture.componentRef.setInput('voice', opts.voice ?? true);
    fixture.componentRef.setInput('card', opts.card ?? false);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    return { el, fixture, label: el.querySelector('.line__label')?.textContent?.trim() };
  }

  const narration = (extra = {}) =>
    line('n1', {
      speakerKind: Kind.NARRATOR,
      text: 'O salão está cheio.',
      canHear: true,
      audioUrl: '/session-messages/n1/audio',
      ...extra,
    });

  it('draws the narration on the page ground with its label, its text and "Ouvir"', () => {
    const { el, label } = render(narration());
    expect(el.classList.contains('line--narration')).toBe(true);
    expect(label).toBe('Narração');
    expect(el.querySelector('.line__text')?.textContent).toBe('O salão está cheio.');
    expect(el.querySelector('app-listen-button button')?.getAttribute('aria-label')).toBe(
      'Ouvir a narração',
    );
  });

  it('on the phone the narration is a white card: surface, 1 px line border, 10 px radius, 12 px padding', () => {
    const { el } = render(narration(), { card: true });
    expect(el.classList.contains('line--card')).toBe(true);
    const style = getComputedStyle(el);
    expect(style.borderTopWidth).toBe('1px');
    expect(style.borderTopStyle).toBe('solid');
    expect(style.paddingTop).toBe('12px');
  });

  it('on a computer the narration stays a borderless block on the page ground', () => {
    const { el } = render(narration());
    expect(el.classList.contains('line--card')).toBe(false);
    expect(getComputedStyle(el).borderTopWidth).not.toBe('1px');
  });

  it('shows the image above the text of a narration', () => {
    const { el } = render(
      narration({
        image: {
          id: 'i1',
          name: 'Salão cheio',
          url: '/images/i1',
          thumbnailUrl: '/images/i1/thumb',
        },
      }),
    );
    const img = el.querySelector('img.line__image') as HTMLImageElement;
    expect(img.getAttribute('src')).toBe('/images/i1');
    expect(img.getAttribute('alt')).toBe('Salão cheio');
  });

  it('hides "Ouvir" when the voice is off or the line cannot be heard', () => {
    expect(render(narration(), { voice: false }).el.querySelector('app-listen-button')).toBeNull();
    expect(render(narration({ canHear: false })).el.querySelector('app-listen-button')).toBeNull();
  });

  it('hides "Ouvir" for the line the server sends with no voice at all (can_hear false, no audio_url)', () => {
    const silent = narration({ canHear: false, audioUrl: '' });
    expect(render(silent).el.querySelector('app-listen-button')).toBeNull();
    const noUrl = narration({ canHear: true, audioUrl: '' });
    expect(render(noUrl).el.querySelector('app-listen-button')).toBeNull();
  });

  it('draws an NPC with a round portrait, a bold name and the text in a bubble', () => {
    const { el, label } = render(
      line('b1', {
        speakerKind: Kind.NPC,
        speakerName: 'Bartolo, o taverneiro',
        text: 'Quartos, só amanhã.',
        canHear: true,
        audioUrl: '/a',
      }),
    );
    expect(el.classList.contains('line--npc')).toBe(true);
    expect(label).toBe('Bartolo, o taverneiro');
    const portrait = el.querySelector('.line__portrait');
    expect(portrait?.textContent?.trim()).toBe('B');
    expect(portrait?.getAttribute('aria-hidden')).toBe('true');
    expect(el.querySelector('.line__bubble')?.textContent).toBe('Quartos, só amanhã.');
    expect(el.querySelector('app-listen-button button')?.getAttribute('aria-label')).toBe(
      'Ouvir a fala de Bartolo, o taverneiro',
    );
  });

  it('uses the NPC portrait image when there is one', () => {
    const { el } = render(
      line('b1', {
        speakerKind: Kind.NPC,
        speakerName: 'Bartolo',
        speakerPortrait: { id: 'x', name: 'x', url: '/images/x', thumbnailUrl: '/images/x/thumb' },
      }),
    );
    expect(el.querySelector('img.line__portrait')?.getAttribute('src')).toBe('/images/x/thumb');
  });

  it('draws the own line to the right, tinted, labelled "<character> · você"', () => {
    const { el, label } = render(
      line('o1', {
        speakerKind: Kind.CHARACTER,
        speakerName: 'Pensantus',
        authorUserId: 'u-p',
        text: 'Gente de quem?',
      }),
    );
    expect(el.classList.contains('line--own')).toBe(true);
    expect(label).toBe('Pensantus · você');
  });

  it("draws the master's own NPC line as an NPC (portrait, bubble, left), labelled '<name> · você'", () => {
    const { el, label } = render(
      line('b2', {
        speakerKind: Kind.NPC,
        speakerName: 'Bartolo, o taverneiro',
        authorUserId: 'u-p',
        text: 'Quartos, só amanhã.',
      }),
      { master: true },
    );
    expect(el.classList.contains('line--npc')).toBe(true);
    expect(el.classList.contains('line--own')).toBe(false);
    expect(label).toBe('Bartolo, o taverneiro · você');
    expect(el.querySelector('.line__portrait')?.textContent?.trim()).toBe('B');
    expect(el.querySelector('.line__bubble')?.textContent).toBe('Quartos, só amanhã.');
  });

  it('draws another player as a bubble without portrait ring text of "você"', () => {
    const { label, el } = render(
      line('o2', {
        speakerKind: Kind.CHARACTER,
        speakerName: 'Brisa',
        authorUserId: 'u-b',
      }),
    );
    expect(el.classList.contains('line--npc')).toBe(true);
    expect(label).toBe('Brisa');
  });

  it('draws an out-of-character remark small and muted, and a removed player by that name', () => {
    const { el, label } = render(
      line('x1', {
        speakerKind: Kind.OUT_OF_CHARACTER,
        speakerName: 'Celso',
        text: 'posso rolar Intuição?',
      }),
    );
    expect(el.classList.contains('line--ooc')).toBe(true);
    expect(label).toBe('Celso · fora do personagem');
    const removed = render(
      line('x2', { speakerKind: Kind.OUT_OF_CHARACTER, speakerName: '', authorRemoved: true }),
    );
    expect(removed.label).toBe('Jogador removido');
  });

  it('draws a whisper with a dashed border and a lock, and tells a player only "só você lê"', () => {
    const { el, label } = render(
      line('w1', {
        speakerKind: Kind.NARRATOR,
        forEveryone: false,
        readerUserIds: ['u-p', 'u-b'],
        text: 'Você reconhece o anel.',
      }),
    );
    expect(el.classList.contains('line--restricted')).toBe(true);
    expect(el.querySelector('.line__label mat-icon')?.textContent?.trim()).toBe('lock');
    expect(label).toContain('Sussurro do mestre · só você lê');
    expect(el.textContent).not.toContain('Brisa');
  });

  it('tells the master who reads a line for chosen readers', () => {
    const { label } = render(
      line('w2', {
        speakerKind: Kind.NARRATOR,
        forEveryone: false,
        readerUserIds: ['u-p', 'u-b'],
      }),
      { master: true },
    );
    expect(label).toContain('Narração · só para Pensantus e Brisa');
  });
});
