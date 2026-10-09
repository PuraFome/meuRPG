import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  EncounterBlockedReason,
  EncounterBlockedSchema,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { MapPointKind, MapPointSchema } from '../../../../../gen/meurpg/maps/v1/maps_pb';
import { AdvantageSourceSchema, RollMode } from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import { SearchForTrapsResponseSchema } from '../../../../../gen/meurpg/play/v1/traps_pb';
import { TrapsClient } from '../../../../core/traps/traps-client';
import { TrapSearchSheet, type TrapSearchData } from './trap-search-sheet';

const roll = (face: number, total: number) => ({
  diceCount: 1,
  diceSides: 20,
  faces: [face],
  modifier: total - face,
  total,
});

function blockedBy(reason: EncounterBlockedReason): ConnectError {
  const err = new ConnectError('x', Code.FailedPrecondition);
  return Object.assign(err, { findDetails: () => [create(EncounterBlockedSchema, { reason })] });
}
const twoDiceError = () => blockedBy(EncounterBlockedReason.SEARCH_NEEDS_TWO_DICE);

describe('TrapSearchSheet', () => {
  function setup(
    responses: (unknown | Error)[],
    listed?: ReturnType<typeof create<typeof MapPointSchema>>[],
  ) {
    const sent: unknown[] = [];
    const keys: string[] = [];
    const extras: (readonly number[] | undefined)[] = [];
    const api = {
      search: async (
        _c: string,
        skill: string,
        die: unknown,
        key: string,
        other = '',
        extra?: readonly number[],
      ) => {
        sent.push(other ? [skill, die, other] : [skill, die]);
        keys.push(key);
        extras.push(extra);
        const next = responses.shift();
        if (next instanceof Error) {
          throw next;
        }
        return next;
      },
    };
    const found = create(MapPointSchema, {
      id: 'x',
      kind: MapPointKind.TRAP,
      name: 'Fosso escondido',
    });
    const data: TrapSearchData = {
      campaignId: 'c',
      skills: {
        perception: 4,
        investigation: 4,
        others: [
          { key: 'skill:acrobatics', name: 'Acrobacia', bonus: 1 },
          { key: 'skill:arcana', name: 'Arcanismo', bonus: 8 },
          { key: 'skill:athletics', name: 'Atletismo', bonus: -1 },
        ],
        reliableTalent: ['skill:acrobatics', 'skill:investigation'],
      },
      diceMode: DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
      state: {
        refresh: async () => undefined,
        points: () => listed ?? [found],
      } as unknown as TrapSearchData['state'],
      inCombat: false,
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: TrapsClient, useValue: api },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(TrapSearchSheet);
    fixture.detectChanges();
    const roller = fixture.componentInstance as unknown as {
      rollWith(d: unknown): Promise<void>;
      pick(s: string): void;
    };
    return { fixture, el: fixture.nativeElement as HTMLElement, sent, keys, extras, roller };
  }

  it('offers Percepção and Investigação with the bonus, the helper line and the three steps', () => {
    const { el } = setup([]);
    const text = el.textContent!.replace(/\s+/g, ' ');
    expect(text).toContain('Percepção +4');
    expect(text).toContain('Investigação +4');
    expect(text).toContain('Algumas armadilhas só se acham com uma perícia específica.');
    expect(text).toContain('Outra perícia…');
    expect(text).toContain('Qualquer outra perícia da sua ficha');
    expect(Array.from(el.querySelectorAll('.steps__name'), (e) => e.textContent)).toEqual([
      'Como',
      'Rolar',
      'Resultado',
    ]);
    expect(el.querySelector('[role=dialog], .frame')).toBeTruthy();
  });

  describe('"Outra perícia…"', () => {
    const choose = (fixture: { detectChanges(): void }, el: HTMLElement) => {
      const radios = el.querySelectorAll<HTMLInputElement>('input[type=radio]');
      radios[2].click();
      radios[2].dispatchEvent(new Event('change'));
      fixture.detectChanges();
    };
    const rows = (el: HTMLElement) =>
      Array.from(el.querySelectorAll('[role=option]'), (r) =>
        r.textContent?.replace(/\s+/g, ' ').trim(),
      );

    it('opens the "Perícia" list with every other skill and the sheet bonus, and rolls nothing until one is picked', () => {
      const { fixture, el } = setup([]);
      expect(el.querySelector('[role=listbox]')).toBeNull();
      choose(fixture, el);
      expect(el.querySelector('#trap-other-cap')?.textContent).toBe('Perícia');
      expect(rows(el)).toEqual(['Acrobacia+1', 'Arcanismo+8', 'Atletismo−1']);
      expect(el.querySelector('.other__value')?.textContent?.trim()).toBe('Escolha uma perícia');
      const roll = Array.from(el.querySelectorAll('button')).find((b) =>
        b.textContent?.includes('Rolar no app'),
      )!;
      expect(roll.disabled).toBe(true);
      expect(el.textContent).toContain(
        'Algumas armadilhas só se acham com uma perícia específica.',
      );
    });

    it("sends OTHER with the skill's key, and writes its name and bonus in the result", async () => {
      const { fixture, el, sent, roller } = setup([
        create(SearchForTrapsResponseSchema, { roll: roll(12, 20), foundPointIds: ['x'] }),
      ]);
      choose(fixture, el);
      (el.querySelectorAll('[role=option]')[1] as HTMLElement).click();
      fixture.detectChanges();
      expect(el.querySelector('.other__value')?.textContent?.trim()).toBe('Arcanismo +8');
      await roller.rollWith({ inApp: true });
      fixture.detectChanges();
      expect(sent[0]).toEqual(['other', { inApp: true }, 'skill:arcana']);
      expect(el.textContent).toContain('1d20 (12) + 8 (Arcanismo) = 20');
      expect(el.textContent).toContain('Achou');
      expect(el.textContent).toContain('Você achou uma armadilha: Fosso escondido.');
    });

    it('picks with the arrows and Enter, the focus on the list', () => {
      const { fixture, el } = setup([]);
      choose(fixture, el);
      const list = el.querySelector<HTMLElement>('[role=listbox]')!;
      const key = (k: string) => {
        list.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true }));
        fixture.detectChanges();
      };
      key('ArrowDown');
      key('Enter');
      expect(el.querySelector('.other__value')?.textContent?.trim()).toBe('Arcanismo +8');
      expect(list.getAttribute('aria-activedescendant')).toBe('trap-other-1');
    });

    it('titles the physical die for the skill picked: "Role 1d20 para Arcanismo (+8)"', () => {
      const { fixture, el } = setup([]);
      choose(fixture, el);
      (el.querySelectorAll('[role=option]')[1] as HTMLElement).click();
      fixture.detectChanges();
      Array.from(el.querySelectorAll('button'))
        .find((b) => b.textContent?.includes('Digitar o resultado'))!
        .click();
      fixture.detectChanges();
      expect(el.querySelector('.type__label')?.textContent).toBe('Role 1d20 para Arcanismo (+8)');
    });

    it('says "Nada" and the same words for a skill no trap accepts as for a roll that fell short', async () => {
      const { fixture, el, roller } = setup([
        create(SearchForTrapsResponseSchema, { roll: roll(18, 19), foundPointIds: [] }),
      ]);
      choose(fixture, el);
      (el.querySelectorAll('[role=option]')[0] as HTMLElement).click();
      fixture.detectChanges();
      await roller.rollWith({ inApp: true });
      fixture.detectChanges();
      expect(el.textContent).toContain('1d20 (18) + 1 (Acrobacia) = 19');
      expect(el.textContent).toContain('Você não encontrou nada.');
      expect(el.querySelector('.res__tag')?.textContent).toContain('Nada');
      expect(el.textContent).not.toContain('Falhou');
    });
  });

  describe('Talento Confiável (PM-03b)', () => {
    it('shows the typed d20 raised in the preview when the sheet has the feature and the skill is proficient', () => {
      const { fixture, el, roller } = setup([]);
      roller.pick('investigation');
      Array.from(el.querySelectorAll('button'))
        .find((b) => b.textContent?.includes('Digitar o resultado'))!
        .click();
      fixture.detectChanges();
      const field = el.querySelector<HTMLInputElement>('#trap-face')!;
      field.value = '6';
      field.dispatchEvent(new Event('input'));
      fixture.detectChanges();
      expect(el.querySelector('.type__sum')?.textContent).toContain(
        '6 → 10 (Talento Confiável) + 4 = 14',
      );
      field.value = '14';
      field.dispatchEvent(new Event('input'));
      fixture.detectChanges();
      expect(el.querySelector('.type__sum')?.textContent).not.toContain('Talento');
    });

    it('shows no raise for a skill the character is not proficient in (Percepção here)', () => {
      const { fixture, el } = setup([]);
      Array.from(el.querySelectorAll('button'))
        .find((b) => b.textContent?.includes('Digitar o resultado'))!
        .click();
      fixture.detectChanges();
      const field = el.querySelector<HTMLInputElement>('#trap-face')!;
      field.value = '6';
      field.dispatchEvent(new Event('input'));
      fixture.detectChanges();
      expect(el.querySelector('.type__sum')?.textContent).toContain('6 + 4 = 10');
      expect(el.querySelector('.type__sum')?.textContent).not.toContain('Talento');
    });

    it("writes the server's raised roll in the result, with the skill the bonus is for", async () => {
      const { fixture, el, roller } = setup([
        create(SearchForTrapsResponseSchema, {
          roll: {
            ...roll(6, 19),
            treatedAs: 10,
            treatedAsSource: 'feature:reliable-talent',
          },
          foundPointIds: [],
        }),
      ]);
      roller.pick('investigation');
      await roller.rollWith({ inApp: true });
      fixture.detectChanges();
      expect(el.querySelector('.res__formula')?.textContent).toBe(
        'd20: 6 → 10 (Talento Confiável) + 13 (Investigação) = 19',
      );
    });
  });

  it('answers the same words for a miss as for no trap', async () => {
    const { fixture, el, roller } = setup([
      create(SearchForTrapsResponseSchema, { roll: roll(6, 13), foundPointIds: [] }),
    ]);
    roller.pick('investigation');
    await roller.rollWith({ face: 6 });
    fixture.detectChanges();
    expect(el.textContent).toContain('Você não encontrou nada.');
    expect(el.textContent).toContain('Nada');
  });

  it('reads the trap found by name', async () => {
    const { fixture, el, roller } = setup([
      create(SearchForTrapsResponseSchema, { roll: roll(13, 17), foundPointIds: ['x'] }),
    ]);
    await roller.rollWith({ inApp: true });
    fixture.detectChanges();
    expect(el.textContent).toContain('Você achou uma armadilha: Fosso escondido.');
    expect(el.textContent).toContain('Ela já aparece no seu mapa.');
  });

  it('asks for the second die when the server says the search has disadvantage, and sends both', async () => {
    const { fixture, el, sent, roller } = setup([
      twoDiceError(),
      create(SearchForTrapsResponseSchema, {
        roll: roll(9, 13),
        secondRoll: roll(4, 8),
        foundPointIds: [],
      }),
    ]);
    await roller.rollWith({ face: 9 });
    fixture.detectChanges();
    expect(el.textContent).toContain('Digite o segundo dado');
    expect(el.textContent).toContain('leva dois d20');
    await roller.rollWith({ face: 4 });
    expect(sent[1]).toEqual(['perception', { face: 9, face2: 4 }]);
  });

  it('asks for the d4 of an effect when the server says the search takes it, and sends it with the d20', async () => {
    const { fixture, el, extras, roller } = setup([
      new ConnectError('the roll takes 1 more die(s): type their faces', Code.InvalidArgument),
      create(SearchForTrapsResponseSchema, { roll: roll(9, 13), foundPointIds: [] }),
    ]);
    (fixture.componentInstance as unknown as { typing: { set(v: boolean): void } }).typing.set(
      true,
    );
    await roller.rollWith({ face: 9 });
    fixture.detectChanges();
    expect(el.textContent).toContain('Esta rolagem leva mais um d4');
    expect(el.querySelector('app-extra-dice label')?.textContent).toContain('Resultado do d4');
    await roller.rollWith({ face: 9 });
    expect(extras).toHaveLength(1);
    fixture.detectChanges();
    expect(el.textContent).toContain('Digite o resultado do d4 antes de confirmar.');
    const d4 = el.querySelector<HTMLInputElement>('app-extra-dice input')!;
    d4.value = '2';
    d4.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    await roller.rollWith({ face: 9 });
    expect(extras[1]).toEqual([2]);
  });

  it('says why the server refused, by reason', async () => {
    for (const [reason, words] of [
      [EncounterBlockedReason.ACTION_USED, 'já usou a sua ação'],
      [EncounterBlockedReason.NOT_YOUR_TURN, 'só na sua vez'],
      [EncounterBlockedReason.TRAP_NOT_ON_MAP, 'não está no mapa'],
      [EncounterBlockedReason.TRAP_SEARCH_NOT_NOW, 'não está na vez de ninguém'],
    ] as const) {
      TestBed.resetTestingModule();
      const { fixture, el, roller } = setup([blockedBy(reason)]);
      await roller.rollWith({ inApp: true });
      fixture.detectChanges();
      expect(el.querySelector('[role=alert]')?.textContent).toContain(words);
    }
  });

  it('says a trap was found, and sends the player to the map, when its name could not be read', async () => {
    const { fixture, el, roller } = setup(
      [create(SearchForTrapsResponseSchema, { roll: roll(13, 17), foundPointIds: ['x'] })],
      [],
    );
    await roller.rollWith({ inApp: true });
    fixture.detectChanges();
    expect(el.textContent).toContain('Você achou uma armadilha.');
    expect(el.textContent).toContain('Veja no seu mapa.');
    expect(el.textContent).not.toContain('Você não encontrou nada.');
  });

  it('keeps the key of a search retried as it was, and takes a new one for another skill', async () => {
    const lost = new ConnectError('lost', Code.Unavailable);
    const { roller, keys } = setup([
      lost,
      lost,
      create(SearchForTrapsResponseSchema, { roll: roll(13, 17), foundPointIds: [] }),
    ]);
    await roller.rollWith({ inApp: true });
    await roller.rollWith({ inApp: true });
    roller.pick('investigation');
    await roller.rollWith({ inApp: true });
    expect(keys[1]).toBe(keys[0]);
    expect(keys[2]).not.toBe(keys[0]);
  });

  it('asks for the second die when the roll takes two d20 for the conditions, and shows the mode, the pair and the sources', async () => {
    const { fixture, el, sent, roller } = setup([
      new ConnectError('the roll takes 2 d20: type 2 face(s) in d20_faces', Code.InvalidArgument),
      create(SearchForTrapsResponseSchema, {
        roll: roll(4, 8),
        secondRoll: roll(15, 19),
        mode: RollMode.DISADVANTAGE,
        sources: [
          create(AdvantageSourceSchema, {
            textPt: 'Envenenado: desvantagem em testes de habilidade',
          }),
        ],
        foundPointIds: [],
      }),
    ]);
    roller.pick('investigation');
    await roller.rollWith({ face: 4 });
    fixture.detectChanges();
    expect(el.querySelector('[role="alert"]')).toBeNull();
    expect(el.textContent).toContain('Digite o segundo dado');
    await roller.rollWith({ face: 15 });
    fixture.detectChanges();
    expect(sent[1]).toEqual(['investigation', { face: 4, face2: 15 }]);
    expect(el.querySelector('.mode__word')?.textContent).toBe('Desvantagem');
    expect(
      Array.from(el.querySelectorAll('.die'), (d) => d.textContent?.replace(/\s+/g, ' ').trim()),
    ).toEqual(['4 conta', '15 não conta']);
    expect(el.querySelector('.mode__sources')?.textContent).toContain('Envenenado');
  });

  it('shows no mode for a normal search', async () => {
    const { fixture, el, roller } = setup([
      create(SearchForTrapsResponseSchema, { roll: roll(6, 13), mode: RollMode.NORMAL }),
    ]);
    await roller.rollWith({ inApp: true });
    fixture.detectChanges();
    expect(el.querySelector('app-check-mode')).toBeNull();
  });
});
