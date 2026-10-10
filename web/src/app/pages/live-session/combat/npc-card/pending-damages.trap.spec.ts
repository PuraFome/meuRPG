import { TestBed } from '@angular/core/testing';

import { CombatantKind, PendingDamageStatus } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { PendingDamages } from './pending-damages';

describe('PendingDamages: the cards of a trap that hits several times', () => {
  const part = (id: string, key: string, pt: string) =>
    ({
      id,
      attackerId: '',
      targetId: 'tie',
      status: PendingDamageStatus.ROLLED,
      diceCount: 1,
      diceSides: 4,
      bonus: 0,
      damageTypeKey: key,
      damageTypePt: pt,
      amount: 3,
      trapPointId: 'pt1',
      roll: { diceCount: 1, diceSides: 4, faces: [3], modifier: 0, total: 3 },
    }) as never;
  const enc = encounter({
    combatants: [combatant({ id: 'tie', label: 'Kai', kind: CombatantKind.PLAYER })],
  } as never);

  it('says which damage type and which of the hits each card is', () => {
    TestBed.configureTestingModule({ providers: [{ provide: CombatClient, useValue: {} }] });
    const fixture = TestBed.createComponent(PendingDamages);
    fixture.componentRef.setInput('pendings', [
      part('a', 'piercing', 'perfurante'),
      part('b', 'poison', 'veneno'),
      part('c', 'piercing', 'perfurante'),
      part('d', 'poison', 'veneno'),
    ]);
    fixture.componentRef.setInput('encounter', enc);
    fixture.componentRef.setInput('campaignId', 'camp');
    fixture.componentRef.setInput('state', new CombatState());
    fixture.detectChanges();
    const labels = Array.from(
      (fixture.nativeElement as HTMLElement).querySelectorAll('[data-testid="trap-part"]'),
    ).map((e) => e.textContent!.trim());
    expect(labels).toEqual([
      'Armadilha: perfurante (1 de 2)',
      'Armadilha: veneno (1 de 2)',
      'Armadilha: perfurante (2 de 2)',
      'Armadilha: veneno (2 de 2)',
    ]);
  });
});
