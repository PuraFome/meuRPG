import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import { CombatantKind, CreatureAttack } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { ActionEconomy, AttackKind, AttackSchema } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { combatant } from '../../../../core/combat/combat-testing';
import { CreaturesClient } from '../../../../core/creatures/creatures-client';
import { FakeCreaturesClient, flat, raven } from '../../../../core/creatures/creatures-testing';
import { CreatureBlock } from './creature-block';

function options(attacks: string[], standard: string[] = []) {
  return {
    options: {
      attacks: attacks.map((key) => ({ enabled: true, attack: create(AttackSchema, { key, name: 'Bite', attackBonus: 5, damage: '2d6+3', damageTypePt: 'perfurante', kind: AttackKind.WEAPON, rangeFt: 5, melee: true }) })),
      standardActions: standard.map((k) => ({ enabled: true, action: { key: `standard:${k}`, namePt: k, economy: ActionEconomy.ACTION } })),
    },
  } as never;
}

describe('CreatureBlock (E9-12 states 3 and 6)', () => {
  function setup(over: Record<string, unknown>, opts: unknown, input: Record<string, unknown> = {}) {
    const api = new FakeCreaturesClient();
    api.blocks.set('monster:dire-wolf', raven({ armorClass: 14 }));
    api.blocks.set('monster:raven', raven());
    TestBed.configureTestingModule({ providers: [{ provide: CreaturesClient, useValue: api }] });
    const fixture = TestBed.createComponent(CreatureBlock);
    fixture.componentRef.setInput('campaignId', 'camp');
    fixture.componentRef.setInput('creature', combatant({ id: 'w1', label: 'Lobo atroz 1', kind: CombatantKind.CREATURE, monsterKey: 'monster:dire-wolf', hitPointsCurrent: 37, hitPointsMax: 37, speedDft: 500, movementLeftDft: 500, speedFt: 15, movementLeftFt: 15, creatureAttack: CreatureAttack.FULL, ...over }));
    fixture.componentRef.setInput('options', opts);
    fixture.componentRef.setInput('acting', true);
    for (const [k, v] of Object.entries(input)) {
      fixture.componentRef.setInput(k, v);
    }
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }
  const settle = async (f: { whenStable(): Promise<unknown>; detectChanges(): void }) => {
    await f.whenStable();
    f.detectChanges();
  };

  it('shows the creature with its numbers, an attack with "Atacar" and "Mover o Lobo atroz 1"', async () => {
    const { fixture, el } = setup({}, options(['monster:dire-wolf#bite']), { joint: true });
    await settle(fixture);
    expect(flat(el.querySelector('.head'))).toContain('Lobo atroz 1 PV 37 de 37 · CA 14');
    expect(flat(el.querySelector('.head'))).toContain('Ainda age');
    expect(flat(el.querySelector('.eco'))).toContain('Ação Disponível');
    expect(flat(el.querySelector('.eco'))).toContain('Movimento 15,0 m de 15,0 m');
    expect(flat(el.querySelector('.atk'))).toContain('Mordida +5 para acertar · 2d6 + 3 perfurante · corpo a corpo, 1,5 m Atacar');
    expect(el.querySelector('.go')?.getAttribute('aria-label')).toBe('Mover o Lobo atroz 1');
  });

  it('asks the attack and the move of its own creature', () => {
    const { fixture, el } = setup({}, options(['monster:dire-wolf#bite']));
    const attacked: string[] = [];
    let moved = 0;
    fixture.componentInstance.attack.subscribe((k) => attacked.push(k));
    fixture.componentInstance.move.subscribe(() => moved++);
    el.querySelector<HTMLButtonElement>('.atk button')!.click();
    el.querySelector<HTMLButtonElement>('.go')!.click();
    expect(attacked).toEqual(['monster:dire-wolf#bite']);
    expect(moved).toBe(1);
  });

  it('a familiar does not attack: the line says so and the standard actions are all it has', () => {
    const { el } = setup({ label: 'Nanquim', monsterKey: 'monster:raven', creatureAttack: CreatureAttack.NONE, speedFlyFt: 50, hitPointsCurrent: 1, hitPointsMax: 1 }, options(['x'], ['dash', 'help']));
    expect(flat(el.querySelector('.note'))).toBe('O familiar não ataca. Ele usa as outras ações:');
    expect(el.querySelector('.atk')).toBeNull();
    expect(Array.from(el.querySelectorAll('.std__btn'), (b) => b.textContent?.trim())).toEqual(['dash', 'help']);
    expect(flat(el.querySelector('.eco'))).toContain('voo');
  });

  it('the familiar of the Pacto da Corrente attacks with its reaction', () => {
    const { el } = setup({ creatureAttack: CreatureAttack.REACTION }, options(['monster:imp#sting']));
    expect(flat(el.querySelector('.atk'))).toContain('Reação');
    expect(el.textContent).not.toContain('O familiar não ataca');
  });

  it('off its turn every button is off, with the reason in a line', () => {
    const { el } = setup({}, options(['monster:dire-wolf#bite']), { acting: false });
    expect(flat(el.querySelector('.why'))).toBe('Ainda não é a vez dela.');
    expect(el.querySelector('.atk button')?.getAttribute('aria-disabled')).toBe('true');
    expect(el.querySelector('.go')?.getAttribute('aria-disabled')).toBe('true');
  });

  it('another player never gets numbers: only the state word', () => {
    const { el } = setup({ hitPointsCurrent: undefined, hitPointsMax: undefined }, null);
    expect(flat(el.querySelector('.head__sub'))).not.toMatch(/PV|\d+ de \d+/);
  });
});
