import { ChangeDetectionStrategy, Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';

import {
  CombatantSchema,
  HitRiderChoice,
  HitRiderKind,
  HitRiderOfferSchema,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { HitRiderCard, type RiderPick } from './hit-rider-card';

const goblin = create(CombatantSchema, { id: 'g1', label: 'Goblin' });

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [HitRiderCard],
  template: `<app-hit-rider-card [offers]="offers" [combatants]="[goblin]" (answer)="picks.push($event)" />`,
})
class Host {
  offers = [
    create(HitRiderOfferSchema, { id: 'r1', kind: HitRiderKind.OPEN_HAND, targetId: 'g1' }),
    create(HitRiderOfferSchema, {
      id: 'r2',
      kind: HitRiderKind.STUNNING_STRIKE,
      targetId: 'g1',
      kiLeft: 0,
    }),
  ];
  goblin = goblin;
  picks: RiderPick[] = [];
}

describe('HitRiderCard', () => {
  it('offers the three Open Hand choices and answers one', () => {
    const f = TestBed.createComponent(Host);
    f.detectChanges();
    const el = f.nativeElement as HTMLElement;
    const text = el.textContent ?? '';
    expect(text).toContain('Derrubar (teste de Destreza)');
    expect(text).toContain('Empurrar até 4,5 m (teste de Força)');
    expect(text).toContain('Sem reações até o fim do seu próximo turno');
    const prone = [...el.querySelectorAll('button')].find((b) =>
      b.textContent?.includes('Derrubar'),
    );
    prone?.click();
    expect(f.componentInstance.picks).toEqual([{ id: 'r1', choice: HitRiderChoice.PRONE }]);
  });

  it('disables Stunning Strike without ki', () => {
    const f = TestBed.createComponent(Host);
    f.detectChanges();
    const stun = [...(f.nativeElement as HTMLElement).querySelectorAll('button')].find((b) =>
      b.textContent?.includes('Golpe Atordoante (1 ki)'),
    );
    expect(stun?.disabled).toBe(true);
  });
});
