import { TestBed } from '@angular/core/testing';
import { MatDialog } from '@angular/material/dialog';
import { of } from 'rxjs';

import { settle } from '../../../../test-settle';

import { SessionMessageSpeakerKind as Kind } from '../../../../gen/meurpg/play/v1/session_messages_pb';
import { ConversationState } from '../../../core/conversation/conversation-state';
import { FakeConversationApi } from '../../../core/conversation/conversation-testing';
import type { ImagePickerData } from '../../../shared/gallery-picker/image-picker-dialog/image-picker-dialog';
import type { ChooserPlayer, Speaker } from './conversation-roster';
import { MasterComposer } from './master-composer';

const SPEAKERS: Speaker[] = [
  { value: '', label: 'Narrador', onStage: false },
  { value: 'npc1', label: 'Bartolo, o taverneiro', onStage: true },
  { value: 'npc2', label: 'Zeca', onStage: false },
];
const PLAYERS: ChooserPlayer[] = [
  { userId: 'u-p', name: 'Pensantus', sub: 'Mago 3' },
  { userId: 'u-b', name: 'Brisa', sub: 'Ladina 2' },
];
const DIRECTORY = new Map([
  ['u-p', 'Pensantus'],
  ['u-b', 'Brisa'],
]);

describe('MasterComposer', () => {
  function setup() {
    const api = new FakeConversationApi();
    const state = new ConversationState(
      api,
      () => 'c1',
      () => 'm',
    );
    const dialog = TestBed.inject(MatDialog);
    const fixture = TestBed.createComponent(MasterComposer);
    fixture.componentRef.setInput('state', state);
    fixture.componentRef.setInput('campaignId', 'c1');
    fixture.componentRef.setInput('speakers', SPEAKERS);
    fixture.componentRef.setInput('players', PLAYERS);
    fixture.componentRef.setInput('directory', DIRECTORY);
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
    const buttonWith = (text: string) =>
      Array.from(el.querySelectorAll('button')).find((b) =>
        b.textContent?.includes(text),
      ) as HTMLButtonElement;
    return { api, state, dialog, fixture, el, area, type, send, buttonWith };
  }

  it('labels the fields "Falar como", "Para" and "Mensagem"', () => {
    const { el } = setup();
    const words = Array.from(el.querySelectorAll('label, .composer__label'), (l) =>
      l.textContent?.trim(),
    );
    expect(words).toEqual(expect.arrayContaining(['Falar como', 'Para', 'Mensagem']));
  });

  it('starts as the narrator, to everyone', async () => {
    const { el, buttonWith, fixture } = setup();
    await settle(fixture);
    expect(el.querySelector('mat-select')?.textContent).toContain('Narrador');
    expect(buttonWith('Todos')).toBeDefined();
  });

  it('lists the narrator, then the NPCs on stage, in the select', async () => {
    const { el, fixture } = setup();
    await settle(fixture);
    (el.querySelector('.mat-mdc-select-trigger') as HTMLElement).click();
    await settle(fixture);
    const options = Array.from(document.querySelectorAll('.mr-speaker-panel mat-option'), (o) =>
      o.textContent?.replace(/\s+/g, ' ').trim(),
    );
    expect(options).toEqual(['Narrador', 'Bartolo, o taverneiro · na cena', 'Zeca']);
  });

  it('narrates by default', () => {
    const { api, type, send } = setup();
    type('O salão está cheio.');
    send();
    expect(api.sent[0]).toMatchObject({
      speakerKind: Kind.NARRATOR,
      speakerId: '',
      readerUserIds: [],
      imageId: '',
    });
  });

  it('speaks as the chosen NPC', async () => {
    const { api, type, send, el, fixture } = setup();
    await settle(fixture);
    (el.querySelector('.mat-mdc-select-trigger') as HTMLElement).click();
    await settle(fixture);
    (document.querySelectorAll('.mr-speaker-panel mat-option')[1] as HTMLElement).click();
    await settle(fixture);
    type('Quartos, só amanhã.');
    send();
    expect(api.sent[0]).toMatchObject({ speakerKind: Kind.NPC, speakerId: 'npc1' });
  });

  it('calls SignalMasterTyping while the master types, at most once in two seconds', () => {
    const { api, type } = setup();
    type('O');
    type('O s');
    type('O sa');
    expect(api.typingCalls).toBe(1);
  });

  it('opens the audience chooser with the current readers and shows the names it answers', () => {
    const { dialog, buttonWith, fixture, type, send, api } = setup();
    const open = vi
      .spyOn(dialog, 'open')
      .mockReturnValue({ afterClosed: () => of(['u-p', 'u-b']) } as never);
    buttonWith('Todos').click();
    fixture.detectChanges();
    const config = open.mock.calls[0][1] as {
      data: { players: ChooserPlayer[]; selected: string[] };
    };
    expect(config.data.players).toEqual(PLAYERS);
    expect(config.data.selected).toEqual([]);
    expect(buttonWith('Pensantus e Brisa')).toBeDefined();
    type('Vocês sentem o mesmo arrepio.');
    send();
    expect(api.sent[0].readerUserIds).toEqual(['u-p', 'u-b']);
  });

  it('keeps the readers when the chooser is cancelled', () => {
    const { dialog, buttonWith, fixture } = setup();
    vi.spyOn(dialog, 'open').mockReturnValue({ afterClosed: () => of(undefined) } as never);
    buttonWith('Todos').click();
    fixture.detectChanges();
    expect(buttonWith('Todos')).toBeDefined();
  });

  it('picks an image with the existing gallery picker, shows it as a chip and sends it', () => {
    const { dialog, buttonWith, fixture, type, send, api, el } = setup();
    const open = vi.spyOn(dialog, 'open').mockReturnValue({ afterClosed: () => of(true) } as never);
    buttonWith('Imagem da galeria').click();
    const data = open.mock.calls[0][1] as { data: ImagePickerData };
    expect(data.data.campaignId).toBe('c1');
    void data.data.submit({ id: 'img1', name: 'Salão cheio' } as never);
    return Promise.resolve().then(() => {
      fixture.detectChanges();
      expect(el.querySelector('.composer__chip')?.textContent).toContain('Salão cheio');
      type('Olhem.');
      send();
      expect(api.sent[0].imageId).toBe('img1');
    });
  });

  it('removes the picked image with the chip button', async () => {
    const { dialog, buttonWith, fixture, el } = setup();
    const open = vi.spyOn(dialog, 'open').mockReturnValue({ afterClosed: () => of(true) } as never);
    buttonWith('Imagem da galeria').click();
    await (open.mock.calls[0][1] as { data: ImagePickerData }).data.submit({
      id: 'img1',
      name: 'Salão cheio',
    } as never);
    fixture.detectChanges();
    (el.querySelector('.composer__chip button') as HTMLButtonElement).click();
    fixture.detectChanges();
    expect(el.querySelector('.composer__chip')).toBeNull();
  });

  it('tells an empty message and keeps the field', () => {
    const { api, send, el } = setup();
    send();
    expect(api.sent).toHaveLength(0);
    expect(el.querySelector('.composer__error')?.textContent).toContain('Escreva uma mensagem.');
  });
});
