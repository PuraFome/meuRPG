import { Code, ConnectError } from '@connectrpc/connect';

import {
  EffectAudience,
  EffectDurationKind,
} from '../../../../gen/meurpg/play/v1/lasting_effects_pb';
import { CombatState } from '../../../core/combat/combat-state';
import { combatant, encounter } from '../../../core/combat/combat-testing';
import { boardEffects, catalogEffect } from '../../../core/effects/effects-testing';
import { AddEffectDialog, type AddEffectData } from './add-effect-dialog';
import { flat, isOff, openDialog } from './effects-dialogs-testing';

describe('AddEffectDialog (W7-E board 4b)', () => {
  const add = vi.fn();
  let data: AddEffectData;

  beforeEach(() => {
    add
      .mockReset()
      .mockResolvedValue({ encounter: encounter({ id: 'enc', revision: 5 }), effects: [] });
    const state = new CombatState();
    state.apply(
      encounter({
        id: 'enc',
        revision: 1,
        combatants: [
          combatant({ id: 'toren', label: 'Toren' }),
          combatant({ id: 'nael', label: 'Nael' }),
          combatant({ id: 'orla', label: 'Orla' }),
        ],
      }),
    );
    data = {
      campaignId: 'camp',
      encounterId: 'enc',
      state,
      catalog: boardEffects().catalog,
      currentCombatantId: 'nael',
    };
  });

  afterEach(() => document.body.replaceChildren());

  it('opens on who is on turn, the first effect of the catalog and the duration it brings', () => {
    const { el } = openDialog(AddEffectDialog, data, { add });
    expect(flat(el.querySelector('h2'))).toBe('Adicionar um efeito');
    const select = (field: string) =>
      el.querySelector(`select[data-field="${field}"]`) as HTMLSelectElement;
    expect(select('target').selectedOptions[0].textContent?.trim()).toBe('Nael');
    expect(Array.from(select('catalog').options).map(flat)).toEqual([
      'Bênção',
      'Derrubado',
      'Velocidade',
    ]);
    expect(select('caster').selectedOptions[0].textContent?.trim()).toBe('Nael');
    expect(select('duration').selectedOptions[0].textContent?.trim()).toBe(
      '10 rodadas, concentração',
    );
    expect(flat(el.querySelector('.cap'))).toBe('Os jogadores veem este efeito');
  });

  it('sends the effect as the catalog has it when the master changes nothing but the target', async () => {
    const { button, settle, pick, close } = openDialog(AddEffectDialog, data, { add });
    await pick('target', 'Toren');
    await pick('catalog', 'Velocidade');
    button('Adicionar')!.click();
    await settle();
    expect(add).toHaveBeenCalledTimes(1);
    const [spec, key] = add.mock.calls[0];
    expect(spec).toMatchObject({
      campaignId: 'camp',
      encounterId: 'enc',
      targetIds: ['toren'],
      catalogKey: 'spell:haste',
      casterId: 'nael',
      duration: { kind: EffectDurationKind.ROUNDS, rounds: 10 },
    });
    expect(spec.playerVisible).toBeUndefined();
    expect(spec.audience).toBeUndefined();
    expect(key).toEqual(expect.any(String));
    expect(close).toHaveBeenCalledWith(true);
    expect(data.state.encounter()?.revision).toBe(5);
  });

  it('starts an effect without a caster when the catalog says it has none', async () => {
    const { el, pick } = openDialog(AddEffectDialog, data, { add });
    await pick('catalog', 'Derrubado');
    expect(el.querySelector('select[data-field="caster"]')).toBeNull();
    expect(
      (
        el.querySelector('select[data-field="duration"]') as HTMLSelectElement
      ).selectedOptions[0].textContent?.trim(),
    ).toBe('Até o mestre encerrar');
  });

  it('puts the same effect on several targets', async () => {
    const { el, button, settle } = openDialog(AddEffectDialog, data, { add });
    const more = Array.from(el.querySelectorAll('.check input')) as HTMLInputElement[];
    expect(more).toHaveLength(2);
    more[0].click();
    more[1].click();
    await settle();
    button('Adicionar')!.click();
    await settle();
    expect(add.mock.calls[0][0].targetIds).toEqual(['nael', 'toren', 'orla']);
  });

  it('takes the number of rounds, the DC and what the players see when the master chooses them', async () => {
    const { button, settle, pick, type, radio } = openDialog(AddEffectDialog, data, { add });
    await pick('duration', 'Outro número de rodadas');
    await type('rounds', '9');
    await type('dc', '14');
    await radio('Sim');
    await radio('Só o dono do alvo');
    button('Adicionar')!.click();
    await settle();
    expect(add.mock.calls[0][0]).toMatchObject({
      duration: { kind: EffectDurationKind.ROUNDS, rounds: 9 },
      saveDc: 14,
      playerVisible: true,
      audience: EffectAudience.OWNER,
    });
  });

  it("sends a duration that ends with someone's turn, and the one that waits for the master", async () => {
    const { button, settle, pick } = openDialog(AddEffectDialog, data, { add });
    await pick('duration', 'Até o fim do turno de alguém');
    await pick('anchor', 'Orla');
    button('Adicionar')!.click();
    await settle();
    expect(add.mock.calls[0][0].duration).toEqual({
      kind: EffectDurationKind.UNTIL_END_OF_TURN_OF,
      rounds: undefined,
      anchorCombatantId: 'orla',
    });
  });

  it('turns the visibility off when the master says no, and keeps the label to the 30 characters', async () => {
    const { el, button, settle, radio } = openDialog(AddEffectDialog, data, { add });
    await radio('Sim');
    const label = el.querySelector('input[maxlength="30"]') as HTMLInputElement;
    expect(label).toBeTruthy();
    label.value = 'x'.repeat(40);
    label.dispatchEvent(new Event('input'));
    await settle();
    button('Adicionar')!.click();
    await settle();
    expect(add.mock.calls[0][0].playerLabel).toHaveLength(30);
    expect(add.mock.calls[0][0].playerVisible).toBe(true);
  });

  it('does not add while a field is out of its range', async () => {
    const { button, settle, pick, type, el } = openDialog(AddEffectDialog, data, { add });
    await pick('duration', 'Outro número de rodadas');
    await type('rounds', '601');
    expect(
      Array.from(el.querySelectorAll('app-field-note')).some((n) =>
        flat(n)?.includes('Digite de 1 a 600 rodadas.'),
      ),
    ).toBe(true);
    expect(isOff(button('Adicionar')!)).toBe(true);
    await type('rounds', '600');
    await type('dc', '41');
    expect(isOff(button('Adicionar')!)).toBe(true);
    await type('dc', '');
    expect(isOff(button('Adicionar')!)).toBe(false);
    button('Adicionar')!.click();
    await settle();
    expect(add).toHaveBeenCalledTimes(1);
  });

  it('keeps the key of a retry, shows the refusal and stays open', async () => {
    add.mockRejectedValueOnce(new ConnectError('x', Code.Unavailable));
    const { button, settle, el, close } = openDialog(AddEffectDialog, data, { add });
    button('Adicionar')!.click();
    await settle();
    expect(flat(el.querySelector('[role="alert"]'))).toContain('Não deu para adicionar o efeito');
    expect(close).not.toHaveBeenCalled();
    button('Adicionar')!.click();
    await settle();
    expect(add.mock.calls[1][1]).toBe(add.mock.calls[0][1]);
    expect(close).toHaveBeenCalledWith(true);
  });

  it('closes with Cancelar and nothing sent', () => {
    const { button, close } = openDialog(AddEffectDialog, data, { add });
    button('Cancelar')!.click();
    expect(close).toHaveBeenCalledWith(false);
    expect(add).not.toHaveBeenCalled();
  });

  it('says so when the catalog did not come', () => {
    const { el, button } = openDialog(AddEffectDialog, { ...data, catalog: [] }, { add });
    expect(flat(el.querySelector('.note'))).toContain('O catálogo de efeitos não veio');
    expect(isOff(button('Adicionar')!)).toBe(true);
  });

  it('asks which ability Aprimorar Habilidade is for and sends it', async () => {
    const enhance = catalogEffect({
      key: 'spell:enhance-ability',
      namePt: 'Aprimorar Habilidade',
      concentration: true,
      hasCaster: true,
    });
    const { el, button, pick, settle } = openDialog(
      AddEffectDialog,
      { ...data, catalog: [...data.catalog, enhance] },
      { add },
    );
    await pick('catalog', 'Bênção');
    expect(el.querySelector('app-ability-picker')).toBeNull();
    await pick('catalog', 'Aprimorar Habilidade');
    const names = Array.from(el.querySelectorAll('app-ability-picker .row__title')).map(flat);
    expect(names).toEqual([
      'Força',
      'Destreza',
      'Constituição',
      'Inteligência',
      'Sabedoria',
      'Carisma',
    ]);
    expect(flat(el.querySelector('app-ability-picker .row__hint'))).toBe('Touro');
    expect(isOff(button('Adicionar')!)).toBe(true);
    const owl = el.querySelectorAll<HTMLInputElement>('app-ability-picker input')[4];
    owl.click();
    await settle();
    expect(isOff(button('Adicionar')!)).toBe(false);
    button('Adicionar')!.click();
    await settle();
    expect(add.mock.calls[0][0]).toMatchObject({
      catalogKey: 'spell:enhance-ability',
      abilityKey: 'wis',
    });
  });
});
