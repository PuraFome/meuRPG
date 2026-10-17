import { TestBed } from '@angular/core/testing';

import { SessionMessageSpeakerKind as Kind } from '../../../../gen/meurpg/play/v1/session_messages_pb';
import { ConversationState } from '../../../core/conversation/conversation-state';
import { FakeConversationApi } from '../../../core/conversation/conversation-testing';
import { PlayerComposer } from './player-composer';

describe('PlayerComposer', () => {
  function setup(characterId = 'pc1') {
    const api = new FakeConversationApi();
    const state = new ConversationState(
      api,
      () => 'c1',
      () => 'u-p',
    );
    const fixture = TestBed.createComponent(PlayerComposer);
    fixture.componentRef.setInput('state', state);
    fixture.componentRef.setInput('characterId', characterId);
    fixture.componentRef.setInput('characterName', 'Pensantus');
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const area = () => el.querySelector('textarea') as HTMLTextAreaElement;
    const type = (text: string) => {
      area().value = text;
      area().dispatchEvent(new Event('input'));
      fixture.detectChanges();
    };
    const send = () => {
      (el.querySelector('button[type="submit"]') as HTMLButtonElement).click();
      fixture.detectChanges();
    };
    const radio = (name: string) =>
      Array.from(el.querySelectorAll<HTMLLabelElement>('label')).find((l) =>
        l.textContent?.includes(name),
      )?.querySelector('input') as HTMLInputElement;
    return { api, state, fixture, el, area, type, send, radio };
  }

  it('labels the field "Mensagem" and the send button "Enviar"', () => {
    const { el } = setup();
    const area = el.querySelector('textarea') as HTMLTextAreaElement;
    expect(el.querySelector(`label[for="${area.id}"]`)?.textContent?.trim()).toBe('Mensagem');
    expect(el.querySelector('button[type="submit"]')?.textContent).toContain('Enviar');
  });

  it('offers "Como <personagem>" and "Fora do personagem" as one radio group', () => {
    const { el } = setup();
    const group = el.querySelector('[role="radiogroup"]');
    expect(group).not.toBeNull();
    const labels = Array.from(el.querySelectorAll('.seg__item'), (l) => l.textContent?.trim());
    expect(labels).toContain('Como Pensantus');
    expect(labels).toContain('Fora do personagem');
  });

  it('sends the line as the character, to everyone, and empties the field', () => {
    const { api, type, send, area } = setup();
    type('  Gente de quem?  ');
    send();
    expect(api.sent).toHaveLength(1);
    expect(api.sent[0]).toMatchObject({
      campaignId: 'c1',
      text: 'Gente de quem?',
      speakerKind: Kind.CHARACTER,
      speakerId: 'pc1',
      onlyMaster: false,
    });
    expect(area().value).toBe('');
  });

  it('sends out of character when that is chosen', () => {
    const { api, type, send, radio, fixture } = setup();
    radio('Fora do personagem').click();
    fixture.detectChanges();
    type('posso rolar Intuição?');
    send();
    expect(api.sent[0]).toMatchObject({ speakerKind: Kind.OUT_OF_CHARACTER, speakerId: '' });
  });

  it('sends only to the master when "Só o mestre" is chosen', () => {
    const { api, type, send, radio, fixture } = setup();
    radio('Só o mestre').click();
    fixture.detectChanges();
    type('Quero ver o anel de perto.');
    send();
    expect(api.sent[0].onlyMaster).toBe(true);
  });

  it('tells an empty message, keeps what was typed and points the field at the error', () => {
    const { api, send, el, area } = setup();
    send();
    expect(api.sent).toHaveLength(0);
    const error = el.querySelector('.composer__error') as HTMLElement;
    expect(error.textContent).toContain('Escreva uma mensagem.');
    expect(area().getAttribute('aria-invalid')).toBe('true');
    expect(area().getAttribute('aria-describedby')).toContain(error.id);
  });

  it('clears the error as soon as the person types', () => {
    const { send, type, el } = setup();
    send();
    type('a');
    expect(el.querySelector('.composer__error')).toBeNull();
  });

  it('shows a counter near the limit and refuses a message over 2.000 characters', () => {
    const { api, type, send, el, area } = setup();
    type('a'.repeat(1700));
    expect(el.querySelector('.composer__count')).toBeNull();
    type('a'.repeat(1850));
    expect(el.querySelector('.composer__count')?.textContent?.trim()).toBe('1.850 / 2.000');
    type('a'.repeat(2001));
    expect(el.querySelector('.composer__error')?.textContent).toContain('2.000');
    expect(area().getAttribute('aria-describedby')).toContain('count');
    send();
    expect(api.sent).toHaveLength(0);
  });

  it('only offers out of character to a player with no living character', () => {
    const { radio, type, send, api } = setup('');
    expect(radio('Como Pensantus').disabled).toBe(true);
    type('oi');
    send();
    expect(api.sent[0].speakerKind).toBe(Kind.OUT_OF_CHARACTER);
  });

  it('sends with Ctrl+Enter', () => {
    const { api, type, area } = setup();
    type('oi');
    area().dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', ctrlKey: true }));
    expect(api.sent).toHaveLength(1);
  });
});
