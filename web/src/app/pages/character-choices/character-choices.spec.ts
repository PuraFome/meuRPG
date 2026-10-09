import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute, Router, provideRouter } from '@angular/router';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';
import { of } from 'rxjs';

import {
  CharacterSchema,
  ChoiceGroupSchema,
  ChoiceKind,
  ChoiceOptionSchema,
  ChoiceOrigin,
  ChoiceRefusalReason,
  ChoiceRefusalSchema,
  ChoiceSchema,
  FullSheetSchema,
  PreviewChoicesResponseSchema,
  type FullSheet,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { DerivedClassSchema, DerivedSheetSchema } from '../../../gen/meurpg/rules/v1/rules_pb';
import { CharacterChoicesClient } from '../../core/characters/character-choices-client';
import { CharacterChoicesPage, QUIET_MS } from './character-choices';

const settle = () => vi.advanceTimersByTimeAsync(QUIET_MS * 2);

function option(key: string, reasonPt = '') {
  return create(ChoiceOptionSchema, {
    key,
    storedKey: `stored-${key}`,
    namePt: key.toUpperCase(),
    summaryPt: `s-${key}`,
    reasonPt,
  });
}

/** The server's answer for a sheet with a half-elf's open choice and one already made, from the keys it is sent. */
function answer(keys: readonly string[]) {
  const pick = (all: string[]) => all.filter((k) => keys.includes(`stored-${k}`));
  const open = pick(['a', 'b', 'c']);
  const made = pick(['x', 'y']);
  const choice = (key: string, picks: number, picked: string[], opts: string[]) =>
    create(ChoiceSchema, {
      key,
      featureKey: `f-${key}`,
      kind: ChoiceKind.OPTIONS,
      titlePt: `Escolha ${key}`,
      labelPt: `Escolha ${key}`,
      picks,
      picked,
      missing: Math.max(picks - picked.length, 0),
      options: opts.map((o) => option(o)),
      resultPt: picked.length > 0 ? `Resultado ${key}` : '',
    });
  return create(PreviewChoicesResponseSchema, {
    groups: [
      create(ChoiceGroupSchema, {
        origin: ChoiceOrigin.RACE,
        sourceNamePt: 'Meio-elfo',
        level: 1,
        choices: [choice('open', 2, open, ['a', 'b', 'c']), choice('made', 1, made, ['x', 'y'])],
      }),
    ],
    done: open.length + made.length,
    total: 3,
  });
}

function character(keys: string[] = ['stored-x']) {
  return create(CharacterSchema, {
    id: 'ch-1',
    name: 'Tharn',
    revision: 7,
    derived: create(DerivedSheetSchema, {
      raceNamePt: 'Meio-elfo',
      classes: [create(DerivedClassSchema, { namePt: 'Guerreiro', level: 1 })],
    }),
    sheet: {
      content: { case: 'full', value: create(FullSheetSchema, { featureChoiceKeys: keys }) },
    },
  });
}

describe('CharacterChoicesPage', () => {
  const client = { character: vi.fn(), preview: vi.fn(), complete: vi.fn() };
  let navigate: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    vi.useFakeTimers();
    Element.prototype.scrollIntoView = vi.fn();
    client.character.mockReset().mockResolvedValue(character());
    client.preview
      .mockReset()
      .mockImplementation((_c: string, _i: string, _s: FullSheet, keys: string[]) =>
        Promise.resolve(answer(keys)),
      );
    client.complete.mockReset().mockResolvedValue(character());
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: CharacterChoicesClient, useValue: client },
        {
          provide: ActivatedRoute,
          useValue: { paramMap: of({ get: (k: string) => (k === 'id' ? 'camp-1' : 'ch-1') }) },
        },
      ],
    });
    navigate = vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
  });

  afterEach(() => vi.useRealTimers());

  async function open(): Promise<{
    fixture: ComponentFixture<CharacterChoicesPage>;
    el: HTMLElement;
  }> {
    const fixture = TestBed.createComponent(CharacterChoicesPage);
    fixture.detectChanges();
    await fixture.whenStable();
    await settle();
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  const cards = (el: HTMLElement) => Array.from(el.querySelectorAll<HTMLElement>('.card'));
  const save = (el: HTMLElement) => el.querySelector<HTMLButtonElement>('.js-save')!;

  it('shows only the choices that are open, with the counter of the whole sheet', async () => {
    const { el } = await open();

    expect(el.querySelector('h1')?.textContent).toContain('Completar escolhas pendentes');
    expect(el.textContent).toContain('Escolha open');
    expect(el.textContent).not.toContain('Escolha made');
    expect(el.textContent).toContain('Escolhas feitas: 1 de 3');
    expect(el.textContent).toContain('Tharn · Guerreiro 1 · Meio-elfo');
    expect(client.preview.mock.calls[0][3]).toEqual(['stored-x']);
  });

  it('asks the server again after a pick, keeping the choice on screen and showing its result line', async () => {
    const { fixture, el } = await open();

    cards(el)[0].click();
    fixture.detectChanges();
    await settle();
    fixture.detectChanges();

    expect(client.preview).toHaveBeenLastCalledWith(
      'camp-1',
      'ch-1',
      expect.anything(),
      ['stored-x', 'stored-a'],
      {},
    );
    expect(el.textContent).toContain('Escolha open');
    expect(el.textContent).toContain('Resultado open');
    expect(el.textContent).toContain('Escolhas feitas: 2 de 3');
  });

  it('saves the picks of the open choice with the revision, then goes back to the sheet', async () => {
    const { fixture, el } = await open();
    cards(el)[0].click();
    cards(el)[1].click();
    fixture.detectChanges();
    await settle();
    fixture.detectChanges();

    save(el).click();
    await settle();

    expect(client.complete).toHaveBeenCalledWith('camp-1', 'ch-1', 7, [
      { choiceKey: 'open', optionKeys: ['a', 'b'], texts: [] },
    ]);
    expect(navigate).toHaveBeenCalledWith(['/campaigns', 'camp-1', 'characters', 'ch-1']);
  });

  it('does not send anything while nothing was picked, and says so', async () => {
    const { fixture, el } = await open();

    save(el).click();
    await settle();
    fixture.detectChanges();

    expect(client.complete).not.toHaveBeenCalled();
    expect(el.querySelector('.js-failure')?.textContent).toContain('pelo menos uma escolha');
  });

  it('says why the rules refused, in place, and stays on the page', async () => {
    client.complete.mockRejectedValue(
      new ConnectError('refused', Code.FailedPrecondition, undefined, [
        {
          desc: ChoiceRefusalSchema,
          value: {
            reason: ChoiceRefusalReason.PREREQUISITE_UNMET,
            issues: [{ labelPt: 'Escolha open' }],
          },
        },
      ]),
    );
    const { fixture, el } = await open();
    cards(el)[0].click();
    fixture.detectChanges();
    await settle();

    save(el).click();
    await settle();
    fixture.detectChanges();

    expect(el.querySelector('.js-failure')?.textContent).toContain('“Escolha open”');
    expect(navigate).not.toHaveBeenCalled();
    expect(save(el).disabled).toBe(false);
  });

  it('says there is nothing to complete when the sheet has no open choice', async () => {
    client.preview.mockImplementation(() =>
      Promise.resolve(create(PreviewChoicesResponseSchema, { groups: [], done: 0, total: 0 })),
    );

    const { el } = await open();

    expect(el.textContent).toContain('Tharn não tem escolhas pendentes');
    expect(el.querySelector('.js-save')).toBeNull();
  });

  it('says the sheet could not be read when the server refuses', async () => {
    client.character.mockRejectedValue(new ConnectError('x', Code.NotFound));

    const { el } = await open();

    expect(el.querySelector('[role="alert"]')?.textContent).toContain('Personagem não encontrado');
  });
});
