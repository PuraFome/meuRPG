import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import { CombatantSchema } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  AttackKind,
  AttackOptionSchema,
  PowerAttackOptionSchema,
  TurnOptionsSchema,
} from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { ActionGroups } from './action-groups';

const heavy = create(PowerAttackOptionSchema, {
  key: 'feat:golpe-pesado@mesa',
  name: 'Golpe Pesado',
  penalty: 5,
  damageBonus: 10,
});

const weapon = (key: string, name: string, kind = AttackKind.WEAPON, trades = [heavy]) =>
  create(AttackOptionSchema, {
    attack: { key, name, namePt: name, kind, attackBonus: 6, damage: '2d6 + 3', rangeFt: 5 },
    enabled: true,
    powerAttacks: trades,
  });

describe('ActionGroups: the power attack line of a weapon row', () => {
  function rows(opts: ReturnType<typeof weapon>[]): HTMLElement[] {
    const fixture = TestBed.createComponent(ActionGroups);
    fixture.componentRef.setInput('options', create(TurnOptionsSchema, { attacks: opts }));
    fixture.componentRef.setInput(
      'own',
      create(CombatantSchema, { movementLeftFt: 25, speedFt: 25 }),
    );
    fixture.detectChanges();
    return Array.from(
      (fixture.nativeElement as HTMLElement).querySelectorAll<HTMLElement>('app-action-row'),
    );
  }
  const line = (r: HTMLElement) => r.querySelector('.row__power');

  it('says the trade under the weapon the server lists it on, with a true minus', () => {
    const [montante, espada, arco] = rows([
      weapon('attack:greatsword', 'Montante'),
      weapon('attack:longsword', 'Espada longa', AttackKind.WEAPON, []),
      weapon('attack:longbow', 'Arco longo', AttackKind.WEAPON, []),
    ]);
    expect(line(montante)).not.toBeNull();
    expect(line(montante)!.textContent!.replace(/\s+/g, ' ').trim()).toContain(
      'Golpe Pesado: −5 no ataque, +10 no dano',
    );
    expect(line(espada)).toBeNull();
    expect(line(arco)).toBeNull();
  });

  it('says nothing under a spell, whatever the list holds', () => {
    const [spell] = rows([weapon('spell:fire-bolt', 'Raio de Fogo', AttackKind.SPELL)]);
    expect(line(spell)).toBeNull();
  });
});
