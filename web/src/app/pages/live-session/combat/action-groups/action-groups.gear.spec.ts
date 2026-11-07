import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import type { GearVm } from '../../../../core/characters/gear';
import { CombatantSchema } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { TurnOptionsSchema } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { ActionGroups } from './action-groups';

const sword: GearVm = {
  key: 'equipment:longsword',
  namePt: 'Espada longa',
  kind: 'weapon',
  equipped: true,
  twoHanded: false,
  armorClassIfToggled: 0,
};
const mace: GearVm = { ...sword, key: 'equipment:mace', namePt: 'Maça', equipped: false };
const shield: GearVm = {
  key: 'equipment:shield',
  namePt: 'Escudo',
  kind: 'shield',
  equipped: false,
  twoHanded: false,
  armorClassIfToggled: 17,
};
const armor: GearVm = {
  key: 'equipment:chain-mail',
  namePt: 'Cota de malha',
  kind: 'armor',
  equipped: true,
  twoHanded: false,
  armorClassIfToggled: 10,
};

describe('ActionGroups: "Sacar ou guardar" (#562)', () => {
  function setup(
    over: { used?: boolean; master?: boolean; locked?: string; gear?: GearVm[] } = {},
  ) {
    const fixture = TestBed.createComponent(ActionGroups);
    fixture.componentRef.setInput('options', create(TurnOptionsSchema, {}));
    fixture.componentRef.setInput(
      'own',
      create(CombatantSchema, {
        movementLeftFt: 30,
        speedFt: 30,
        objectInteractionUsed: over.used ?? false,
      }),
    );
    fixture.componentRef.setInput('gear', over.gear ?? [sword, mace, shield, armor]);
    fixture.componentRef.setInput('gearArmorClass', 16);
    fixture.componentRef.setInput('gearMaster', over.master ?? false);
    fixture.componentRef.setInput('locked', over.locked ?? '');
    const asked: { key: string; equipped: boolean }[] = [];
    fixture.componentInstance.gearToggled.subscribe((t) => asked.push(t));
    fixture.detectChanges();
    const head = Array.from(
      (fixture.nativeElement as HTMLElement).querySelectorAll<HTMLButtonElement>(
        'button[aria-expanded]',
      ),
    ).find((b) => b.textContent?.includes('Sacar ou guardar'));
    head?.click();
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement, asked };
  }

  const costs = (el: HTMLElement) =>
    Array.from(el.querySelectorAll('.gear-row')).map((r) => ({
      name: r.querySelector('.gear-row__name')!.textContent!.trim(),
      cost: r.querySelector('.gear-row__cost')?.textContent?.trim() ?? '',
      blocked: r.querySelector('.gear-row__blocked')?.textContent?.trim() ?? '',
    }));

  it('puts the list in a fold named "Sacar ou guardar"', () => {
    const { el } = setup();
    expect(el.textContent).toContain('Sacar ou guardar');
  });

  it('says the first weapon uses the turn’s free interaction, a shield the action, and armor not at all', () => {
    const { el } = setup();
    const rows = costs(el);
    expect(rows.find((r) => r.name === 'Espada longa')!.cost).toBe(
      'Usa a interação gratuita do turno.',
    );
    expect(rows.find((r) => r.name === 'Escudo')!.cost).toBe('Usa a sua ação.');
    expect(rows.find((r) => r.name === 'Cota de malha')!.blocked).toContain(
      'não dá em pleno combate',
    );
    expect(el.querySelector('[data-gear-key="equipment:chain-mail"]')).toBeNull();
  });

  it('says a second weapon uses the action once the free interaction is spent', () => {
    const { el } = setup({ used: true });
    expect(costs(el).find((r) => r.name === 'Maça')!.cost).toBe('Usa a sua ação.');
  });

  it('tells the master it costs nothing, and leaves armor to them', () => {
    const { el } = setup({ master: true });
    const rows = costs(el);
    expect(
      rows.every((r) => (r.name === 'Cota de malha' ? true : r.cost.startsWith('Sem custo'))),
    ).toBe(true);
    expect(el.querySelector('[data-gear-key="equipment:chain-mail"]')).not.toBeNull();
  });

  it('asks for the opposite state of the piece and shows the server’s armor class on the shield', () => {
    const { el, asked } = setup();
    const button = el.querySelector<HTMLButtonElement>('[data-gear-key="equipment:shield"]')!;
    expect(button.textContent).toContain('CA 17');
    button.click();
    expect(asked).toEqual([{ key: 'equipment:shield', equipped: true }]);
  });

  it('has no buttons while the turn is held for something else', () => {
    const { el } = setup({ locked: 'Aguardando o mestre' });
    expect(el.querySelector('.gear-row__button')).toBeNull();
  });

  it('shows no fold without gear', () => {
    const { el } = setup({ gear: [] });
    expect(el.textContent).not.toContain('Sacar ou guardar');
  });
});
