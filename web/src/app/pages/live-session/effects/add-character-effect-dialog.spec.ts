import { Code, ConnectError } from '@connectrpc/connect';

import {
  EffectAudience,
  EffectDurationKind,
} from '../../../../gen/meurpg/play/v1/lasting_effects_pb';
import { catalogEffect } from '../../../core/effects/effects-testing';
import {
  AddCharacterEffectDialog,
  type AddCharacterEffectData,
} from './add-character-effect-dialog';
import { flat, isOff, openDialog } from './effects-dialogs-testing';

describe('AddCharacterEffectDialog ("Dar efeito")', () => {
  const addCharacterEffect = vi.fn();
  let data: AddCharacterEffectData;

  beforeEach(() => {
    addCharacterEffect.mockReset().mockResolvedValue({ effectIds: ['e1'] });
    data = {
      campaignId: 'camp',
      characters: [
        { id: 'ch1', name: 'Toren' },
        { id: 'ch2', name: 'Ragna' },
      ],
      catalog: [
        catalogEffect({ key: 'spell:bless', namePt: 'Bênção', concentration: true }),
        catalogEffect({
          key: 'spell:enhance-ability',
          namePt: 'Aprimorar Habilidade',
          concentration: true,
        }),
        catalogEffect({
          key: 'condition:prone',
          namePt: 'Derrubado',
          defaultDurationKind: EffectDurationKind.UNTIL_DISMISSED,
          defaultRounds: 0,
        }),
      ],
    };
  });

  afterEach(() => document.body.replaceChildren());

  const checks = (el: HTMLElement) =>
    Array.from(el.querySelectorAll<HTMLInputElement>('[data-testid="give-characters"] input'));

  it('lists the characters and the catalog, and gives nothing until a character is ticked', async () => {
    const { el, button, settle, pick } = openDialog(AddCharacterEffectDialog, data, {
      addCharacterEffect,
    });
    expect(flat(el.querySelector('h2'))).toBe('Dar um efeito');
    expect(checks(el)).toHaveLength(2);
    const catalog = el.querySelector('select[data-field="catalog"]') as HTMLSelectElement;
    expect(Array.from(catalog.options).map(flat)).toEqual([
      'Aprimorar Habilidade',
      'Bênção',
      'Derrubado',
    ]);
    await pick('catalog', 'Bênção');
    expect(isOff(button('Dar efeito')!)).toBe(true);
    checks(el)[0].click();
    await settle();
    expect(isOff(button('Dar efeito')!)).toBe(false);
  });

  it('gives the effect with the catalog duration to the characters ticked', async () => {
    const { el, button, settle, pick, close } = openDialog(AddCharacterEffectDialog, data, {
      addCharacterEffect,
    });
    await pick('catalog', 'Bênção');
    checks(el)[0].click();
    checks(el)[1].click();
    await settle();
    button('Dar efeito')!.click();
    await settle();
    const [spec, key] = addCharacterEffect.mock.calls[0];
    expect(spec).toMatchObject({
      campaignId: 'camp',
      characterIds: ['ch1', 'ch2'],
      catalogKey: 'spell:bless',
      durationKind: EffectDurationKind.UNSPECIFIED,
      seconds: 0,
    });
    expect(spec.playerVisible).toBeUndefined();
    expect(key).toEqual(expect.any(String));
    expect(close).toHaveBeenCalledWith({
      effectName: 'Bênção',
      characterNames: ['Toren', 'Ragna'],
    });
  });

  it('gives it for a time of game time, a preset or any number of seconds', async () => {
    const { el, button, settle, pick, type } = openDialog(AddCharacterEffectDialog, data, {
      addCharacterEffect,
    });
    await pick('catalog', 'Bênção');
    checks(el)[0].click();
    await pick('duration', 'Um tempo de jogo');
    await pick('time', '1 hora');
    expect(flat(el.querySelector('[data-testid="give-preview"]'))).toBe(
      'Dura 1 hora de tempo de jogo.',
    );
    button('Dar efeito')!.click();
    await settle();
    expect(addCharacterEffect.mock.calls[0][0]).toMatchObject({
      durationKind: EffectDurationKind.ROUNDS,
      seconds: 3600,
    });

    await pick('time', 'Outro tempo');
    await type('seconds', '86401');
    expect(isOff(button('Dar efeito')!)).toBe(true);
    await type('seconds', '90');
    button('Dar efeito')!.click();
    await settle();
    expect(addCharacterEffect.mock.calls[1][0]).toMatchObject({
      durationKind: EffectDurationKind.ROUNDS,
      seconds: 90,
    });
  });

  it('gives it until dismissed or until a long rest', async () => {
    const { el, button, settle, pick } = openDialog(AddCharacterEffectDialog, data, {
      addCharacterEffect,
    });
    await pick('catalog', 'Derrubado');
    checks(el)[1].click();
    await pick('duration', 'Até dispensar');
    button('Dar efeito')!.click();
    await settle();
    await pick('duration', 'Até um descanso longo');
    button('Dar efeito')!.click();
    await settle();
    expect(addCharacterEffect.mock.calls.map((c) => c[0].durationKind)).toEqual([
      EffectDurationKind.UNTIL_DISMISSED,
      EffectDurationKind.LONG_REST,
    ]);
  });

  it('takes what the players see when the master chooses it', async () => {
    const { el, button, settle, pick, radio } = openDialog(AddCharacterEffectDialog, data, {
      addCharacterEffect,
    });
    await pick('catalog', 'Bênção');
    checks(el)[0].click();
    await radio('Sim');
    await radio('Só o dono do alvo');
    const label = el.querySelector('input[maxlength="30"]') as HTMLInputElement;
    label.value = 'Abençoado';
    label.dispatchEvent(new Event('input'));
    await settle();
    button('Dar efeito')!.click();
    await settle();
    expect(addCharacterEffect.mock.calls[0][0]).toMatchObject({
      playerVisible: true,
      audience: EffectAudience.OWNER,
      playerLabel: 'Abençoado',
    });
  });

  it('asks which ability Aprimorar Habilidade is for and sends it', async () => {
    const { el, button, settle, pick } = openDialog(AddCharacterEffectDialog, data, {
      addCharacterEffect,
    });
    checks(el)[0].click();
    await pick('catalog', 'Aprimorar Habilidade');
    expect(el.querySelectorAll('app-ability-picker input')).toHaveLength(6);
    expect(isOff(button('Dar efeito')!)).toBe(true);
    el.querySelectorAll<HTMLInputElement>('app-ability-picker input')[2].click();
    await settle();
    button('Dar efeito')!.click();
    await settle();
    expect(addCharacterEffect.mock.calls[0][0]).toMatchObject({
      catalogKey: 'spell:enhance-ability',
      abilityKey: 'con',
    });
  });

  it('stops at ten characters', async () => {
    const many = Array.from({ length: 11 }, (_, i) => ({ id: `c${i}`, name: `P${i}` }));
    const { el, settle } = openDialog(
      AddCharacterEffectDialog,
      { ...data, characters: many },
      {
        addCharacterEffect,
      },
    );
    checks(el)
      .slice(0, 10)
      .forEach((c) => c.click());
    await settle();
    expect(checks(el)[10].disabled).toBe(true);
  });

  it('says a character is in a combat, keeps the key of a retry and stays open', async () => {
    addCharacterEffect.mockRejectedValueOnce(new ConnectError('x', Code.FailedPrecondition));
    const { el, button, settle, pick, close } = openDialog(AddCharacterEffectDialog, data, {
      addCharacterEffect,
    });
    await pick('catalog', 'Bênção');
    checks(el)[0].click();
    await settle();
    button('Dar efeito')!.click();
    await settle();
    expect(flat(el.querySelector('[role="alert"] p'))).toBe(
      'Um dos personagens está em um combate. Dê o efeito por lá, em "Adicionar efeito".',
    );
    expect(close).not.toHaveBeenCalled();
    button('Dar efeito')!.click();
    await settle();
    expect(addCharacterEffect.mock.calls[1][1]).toBe(addCharacterEffect.mock.calls[0][1]);
    expect(close).toHaveBeenCalled();
  });

  it('closes with Cancelar and nothing sent', () => {
    const { button, close } = openDialog(AddCharacterEffectDialog, data, { addCharacterEffect });
    button('Cancelar')!.click();
    expect(close).toHaveBeenCalledWith(null);
    expect(addCharacterEffect).not.toHaveBeenCalled();
  });
});
