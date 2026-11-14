import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  CombatantState,
  EncounterBlockedReason,
  EncounterBlockedSchema,
  EncounterSchema,
  ResourceTargetSchema,
  TargetInReachSchema,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  ActionEconomy,
  ActionOptionSchema,
  FeatureTargetWho,
  UseScope,
} from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { CombatState } from '../../../../core/combat/combat-state';
import { encounter } from '../../../../core/combat/combat-testing';
import { UseFeatureSheet, type UseFeatureSheetData } from './use-feature-sheet';

const flat = (n: Element | null | undefined) =>
  n?.textContent?.replace(/\s+/g, ' ').replace(/ /g, ' ').trim() ?? '';

const target = (id: string, label: string, reason = '', feet = 5) =>
  create(ResourceTargetSchema, {
    target: create(TargetInReachSchema, {
      combatantId: id,
      label,
      state: CombatantState.UNHURT,
      distanceFt: feet,
    }),
    disabledReasonPt: reason,
  });

const option = (over: Record<string, unknown> = {}, action: Record<string, unknown> = {}) =>
  create(ActionOptionSchema, {
    enabled: true,
    viaUseFeature: true,
    action: {
      key: 'feature:x',
      namePt: 'Corrigir o tipo de dano',
      economy: ActionEconomy.FREE,
      ...action,
    },
    ...over,
  } as never);

const FIRE = {
  choices: [
    { key: 'acid', textPt: 'Ácido' },
    { key: 'fire', textPt: 'Fogo' },
  ],
};

describe('UseFeatureSheet (#743)', () => {
  const useFeature = vi.fn();
  const close = vi.fn();

  function render(over: Partial<UseFeatureSheetData> = {}) {
    const state = new CombatState();
    state.apply(encounter());
    const data: UseFeatureSheetData = {
      campaignId: 'camp',
      encounterId: 'enc',
      combatantId: 'pensantus',
      userLabel: 'Pensantus',
      option: option(FIRE),
      targets: [],
      resource: undefined,
      master: false,
      state,
      ...over,
    };
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({
      providers: [
        { provide: CombatClient, useValue: { features: { useFeature } } },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close, disableClose: false } },
      ],
    });
    const fixture = TestBed.createComponent(UseFeatureSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const settle = async () => {
      await fixture.whenStable();
      fixture.detectChanges();
    };
    const button = (text: string) =>
      Array.from(el.querySelectorAll('button')).find((b) =>
        flat(b)
          .replace(/^check ?/, '')
          .startsWith(text),
      )!;
    const click = (b: HTMLElement) => {
      b.click();
      fixture.detectChanges();
    };
    const box = (label: string) =>
      Array.from(
        el.querySelectorAll<HTMLInputElement>('input[type=checkbox], input[type=radio]'),
      ).find((i) => flat(i.closest('label')).includes(label))!;
    const toggle = (label: string) => {
      const input = box(label);
      input.checked = !input.checked;
      input.dispatchEvent(new Event('change'));
      fixture.detectChanges();
    };
    return { fixture, el, settle, button, click, toggle, box, state };
  }

  beforeEach(() => {
    useFeature.mockReset();
    close.mockReset();
  });

  it('is titled with the feature and says its economy and its limit', () => {
    const { el } = render({
      option: option({ ...FIRE, useScope: UseScope.TURN, useTotal: 1, useLeft: 1 }),
    });
    expect(flat(el.querySelector('h2'))).toBe('Corrigir o tipo de dano');
    expect(flat(el.querySelector('.frame__sub'))).toBe('Sem custo · 1 por turno · restam 1 de 1');
  });

  it('keeps "Usar" off until an option is chosen, and says why', () => {
    const { el, button } = render();
    expect(flat(el.querySelector('.choices .ask'))).toBe('Qual opção');
    expect(button('Usar').getAttribute('aria-disabled')).toBe('true');
    expect(flat(el.querySelector('#use-missing'))).toContain('Escolha uma opção.');
    expect(button('Usar').getAttribute('aria-describedby')).toBe('use-missing');
  });

  it('marks the chosen option, and the button says what will be sent', () => {
    const { el, button, click } = render();
    click(button('Fogo'));
    expect(button('Fogo').getAttribute('aria-pressed')).toBe('true');
    expect(button('Ácido').getAttribute('aria-pressed')).toBe('false');
    expect(el.querySelector('#use-missing')).toBeNull();
    expect(flat(button('Usar'))).toBe('Usar: Fogo');
  });

  it('uses the feature with the choice and the key, applies the combat and closes with what it gave', async () => {
    useFeature.mockResolvedValue({
      encounter: create(EncounterSchema, { id: 'enc', revision: 9 }),
      healed: 0,
      tempHp: 16,
    });
    const { button, click, settle, state } = render();
    click(button('Fogo'));
    click(button('Usar: Fogo'));
    await settle();
    expect(useFeature).toHaveBeenCalledWith(
      {
        campaignId: 'camp',
        encounterId: 'enc',
        combatantId: 'pensantus',
        actionKey: 'feature:x',
        targetIds: [],
        choice: 'fire',
      },
      expect.any(String),
    );
    expect(state.encounter()?.revision).toBe(9);
    expect(close).toHaveBeenCalledWith({ healed: 0, tempHp: 16 });
  });

  it('stops both buttons while it is calling, and does not close under the call', async () => {
    let finish: (v: unknown) => void = () => undefined;
    useFeature.mockReturnValue(new Promise((resolve) => (finish = resolve)));
    const { fixture, button, click, settle } = render();
    click(button('Fogo'));
    click(button('Usar: Fogo'));
    expect(flat(button('Usando…'))).toBe('Usando…');
    expect(button('Cancelar').disabled).toBe(true);
    expect(button('Usando…').getAttribute('aria-disabled')).toBe('true');
    click(button('Cancelar'));
    expect(close).not.toHaveBeenCalled();
    finish({ encounter: create(EncounterSchema, { id: 'enc' }), healed: 0, tempHp: 0 });
    await settle();
    fixture.detectChanges();
    expect(close).toHaveBeenCalledTimes(1);
  });

  it("writes the server's refusal in a red band, names who, and keeps what was chosen", async () => {
    const refused = new ConnectError('x', Code.FailedPrecondition, undefined, [
      {
        desc: EncounterBlockedSchema,
        value: create(EncounterBlockedSchema, {
          reason: EncounterBlockedReason.FEATURE_TARGET,
          combatantIds: ['z1'],
        }),
      },
    ]);
    useFeature.mockRejectedValue(refused);
    const o = option({
      targetMin: 1,
      targetMax: 1,
      targetWho: FeatureTargetWho.ENEMY,
      targetRangeFt: 10,
    });
    const { el, button, click, toggle, box, settle } = render({
      option: o,
      targets: [target('z1', 'Zumbi 1'), target('z2', 'Zumbi 2')],
    });
    toggle('Zumbi 1');
    click(button('Usar no Zumbi 1'));
    await settle();
    const band = el.querySelector('[role=alert]')!;
    expect(flat(band)).toContain('O Zumbi 1 saiu do alcance. Escolha outro alvo.');
    expect(flat(band.querySelector('b'))).toBe('O Zumbi 1 saiu do alcance.');
    expect(box('Zumbi 1').checked).toBe(true);
    expect(close).not.toHaveBeenCalled();
    expect(flat(button('Usar no Zumbi 1'))).toBe('Usar no Zumbi 1');
  });

  it('says a refused choice with the general sentence, and sends the same key on a retry', async () => {
    useFeature.mockRejectedValue(
      new ConnectError('x', Code.FailedPrecondition, undefined, [
        {
          desc: EncounterBlockedSchema,
          value: create(EncounterBlockedSchema, { reason: EncounterBlockedReason.FEATURE_CHOICE }),
        },
      ]),
    );
    const { el, button, click, settle } = render();
    click(button('Fogo'));
    click(button('Usar: Fogo'));
    await settle();
    expect(flat(el.querySelector('[role=alert]'))).toContain(
      'Essa opção não vale mais. Escolha outra.',
    );
    click(button('Usar: Fogo'));
    await settle();
    const keys = useFeature.mock.calls.map((c) => c[1]);
    expect(keys[0]).toBe(keys[1]);
  });

  describe('a use for several creatures', () => {
    const o = () =>
      option(
        { targetMin: 1, targetMax: 2, targetWho: FeatureTargetWho.ALLY, targetRangeFt: 30 },
        { namePt: 'Bênção', economy: ActionEconomy.ACTION },
      );
    const allies = [
      target('tor', 'Tor'),
      target('brisa', 'Brisa'),
      target('pens', 'Pensantus', 'Fora de alcance: até 9 m', 40),
      target('jabuti', 'Jabuti', 'Caído'),
    ];

    it('heads the list with the limit, as check boxes, with the count and the reasons of the server', () => {
      const { el, box } = render({ option: o(), targets: allies });
      expect(flat(el.querySelector('legend'))).toBe('Em quem · de 1 a 2 aliados até 9 m');
      expect(el.querySelectorAll('input[type=checkbox]').length).toBe(4);
      expect(box('Pensantus').disabled).toBe(true);
      expect(flat(box('Pensantus').closest('label'))).toContain('Fora de alcance: até 9 m');
      expect(flat(box('Jabuti').closest('label'))).toContain('Caído');
      expect(flat(el.querySelector('p[role=status]'))).toBe(
        '0 de 2 escolhidos. Criaturas escondidas nunca aparecem aqui.',
      );
    });

    it('needs at least one, takes no more than the limit and sends the ids in the order picked', async () => {
      useFeature.mockResolvedValue({
        encounter: create(EncounterSchema, { id: 'enc' }),
        healed: 0,
        tempHp: 0,
      });
      const { el, button, click, toggle, box, settle } = render({
        option: o(),
        targets: [...allies, target('wight', 'Wight')],
      });
      expect(flat(el.querySelector('#use-missing'))).toContain('Escolha de 1 a 2 aliados até 9 m.');
      toggle('Brisa');
      toggle('Tor');
      toggle('Wight');
      expect(box('Wight').disabled).toBe(true);
      expect(flat(el.querySelector('p[role=status]'))).toContain('2 de 2 escolhidos.');
      expect(flat(button('Usar'))).toBe('Usar em 2');
      click(button('Usar em 2'));
      await settle();
      expect(useFeature.mock.calls[0][0]).toMatchObject({ targetIds: ['brisa', 'tor'] });
      toggle('Brisa');
      expect(box('Wight').disabled).toBe(false);
    });

    it('lets the player take one off again', () => {
      const { el, toggle, box } = render({ option: o(), targets: allies });
      toggle('Tor');
      expect(box('Tor').checked).toBe(true);
      toggle('Tor');
      expect(box('Tor').checked).toBe(false);
      expect(flat(el.querySelector('p[role=status]'))).toContain('0 de 2');
    });

    it('scrolls inside itself from eight creatures, and keeps the buttons out of the scroll', () => {
      const many = Array.from({ length: 8 }, (_, i) => target(`c${i}`, `Aliado ${i + 1}`));
      const { el } = render({ option: o(), targets: many });
      const list = el.querySelector('.pick__list');
      expect(list?.classList.contains('pick__list--scrolls')).toBe(true);
      expect(list?.getAttribute('tabindex')).toBe('0');
      expect(list?.querySelector('button')).toBeNull();
      const few = render({ option: o(), targets: allies });
      expect(few.el.querySelector('.pick__list')?.classList.contains('pick__list--scrolls')).toBe(
        false,
      );
    });
  });

  describe('for one creature', () => {
    it('is a list of radio cards, and says "Usar no Zumbi 1"', () => {
      const one = option(
        { targetMin: 1, targetMax: 1, targetWho: FeatureTargetWho.ENEMY, targetRangeFt: 10 },
        { namePt: 'Halo de esporos', economy: ActionEconomy.REACTION },
      );
      const { el, button, toggle } = render({
        option: one,
        targets: [target('z1', 'Zumbi 1'), target('z2', 'Zumbi 2', 'Fora de alcance: até 3 m', 15)],
      });
      expect(flat(el.querySelector('legend'))).toBe('Em quem · 1 inimigo até 3 m');
      expect(el.querySelectorAll('input[type=radio]').length).toBe(2);
      expect(el.querySelector('p[role=status]')).toBeNull();
      toggle('Zumbi 1');
      expect(flat(button('Usar'))).toBe('Usar no Zumbi 1');
    });
  });

  describe('for the master acting as the character', () => {
    it('confirms a use with nothing to choose: whom it is for, what it gives, "Usar"', () => {
      const { el, button } = render({
        master: true,
        userLabel: 'Sálvia',
        option: option(
          { useScope: UseScope.COMBAT, useTotal: 1, useLeft: 1, usesLeft: 2, resourceTotal: 2 },
          {
            namePt: 'Entidade simbiótica',
            economy: ActionEconomy.ACTION,
            resourceKey: 'wild_shape',
            tempHp: 16,
          },
        ),
        resource: {
          key: 'wild_shape',
          namePt: 'Forma Selvagem',
          total: 2,
          used: 0,
          recharge: 'short_rest',
        },
      });
      expect(flat(el.querySelector('.frame__sub'))).toBe(
        'Ação · 1 por combate · restam 1 de 1 · gasta 1 uso de Forma Selvagem (restam 2 de 2) · 16 PV temporários',
      );
      expect(flat(el.querySelector('.who b'))).toBe('Em quem: Sálvia');
      expect(flat(el.querySelector('.who .small'))).toBe('A característica é só em quem usa.');
      expect(flat(el.querySelector('.what'))).toBe('Sálvia ganha 16 PV temporários.');
      expect(button('Usar').getAttribute('aria-disabled')).toBeNull();
    });

    it('does not say that hidden creatures never show: the master sees them all', () => {
      const { el } = render({
        master: true,
        option: option({ targetMin: 1, targetMax: 2, targetWho: FeatureTargetWho.ANY }),
        targets: [target('a', 'Ana'), target('b', 'Bia')],
      });
      expect(flat(el.querySelector('p[role=status]'))).toBe('0 de 2 escolhidos.');
    });
  });
});
