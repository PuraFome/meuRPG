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
import {
  Ability,
  CreatureSchema,
  SpellSaveSuccess,
} from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { creatureAction, creaturesProvider } from '../../../../core/combat/combat-testing';
import { settle } from '../../../../../test-settle';
import { HitRiderCard, type RiderPick } from './hit-rider-card';

const brisa = create(CombatantSchema, { id: 'brisa', label: 'Brisa' });
const ghoul = create(CombatantSchema, {
  id: 'ghoul',
  label: 'Carniçal 1',
  bestiaryCreatureKey: 'monster:ghoul',
});

const spider = create(CombatantSchema, {
  id: 'spider',
  label: 'Aranha 1',
  bestiaryCreatureKey: 'monster:giant-spider',
});

const claws = create(HitRiderOfferSchema, {
  id: 'r1',
  kind: HitRiderKind.CREATURE,
  attackerId: 'ghoul',
  targetId: 'brisa',
  saveDc: 10,
  actionKey: 'monster:ghoul#claws',
  actionNamePt: 'Garras',
  conditionKey: 'condition:paralyzed',
  conditionNamePt: 'Paralisado',
});
const grab = create(HitRiderOfferSchema, {
  id: 'r2',
  kind: HitRiderKind.CREATURE,
  attackerId: 'spider',
  targetId: 'brisa',
  actionKey: 'monster:giant-spider#web',
  actionNamePt: 'Teia',
  conditionKey: 'condition:grappled',
  conditionNamePt: 'Agarrado',
  escapeDc: 12,
});

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [HitRiderCard],
  template: `<app-hit-rider-card
    campaignId="c"
    [offers]="offers"
    [combatants]="[brisa, ghoul, spider]"
    (answer)="picks.push($event)"
  />`,
})
class Host {
  offers = [claws, grab];
  brisa = brisa;
  ghoul = ghoul;
  spider = spider;
  picks: RiderPick[] = [];
}

async function open() {
  TestBed.resetTestingModule();
  TestBed.configureTestingModule({
    providers: [
      creaturesProvider({
        'monster:ghoul': create(CreatureSchema, {
          actions: [
            creatureAction({
              key: 'monster:ghoul#claws',
              text: 'Hit: the target must succeed on a DC 10 Constitution saving throw or be paralyzed. This effect ignores elves and undead.',
              save: {
                ability: Ability.CONSTITUTION,
                dc: 10,
                onSuccess: SpellSaveSuccess.NONE,
                damage: [],
              },
            } as never),
          ],
        }),
        'monster:giant-spider': create(CreatureSchema, {
          actions: [
            creatureAction({
              key: 'monster:giant-spider#web',
              text: 'Ranged attack. Hit: the target is restrained.',
              hitCondition: { conditionKey: 'condition:grappled', escapeDc: 12, maxSize: 'Large' },
            } as never),
          ],
        }),
      }),
    ],
  });
  const fixture = TestBed.createComponent(Host);
  await settle(fixture);
  await settle(fixture);
  const root = fixture.nativeElement as HTMLElement;
  const groups = [...root.querySelectorAll<HTMLElement>('[data-testid="hit-rider"]')];
  const button = (group: HTMLElement, label: string) =>
    [...group.querySelectorAll('button')].find((b) => b.textContent?.trim() === label)!;
  return { fixture, groups, button };
}

describe("HitRiderCard: a creature's rider", () => {
  it('offers the save with the target, the DC and the ability of the stat block, and what a failure gives', async () => {
    const { groups } = await open();
    const text = (groups[0].textContent ?? '').replace(/\s+/g, ' ');
    expect(groups[0].getAttribute('aria-label')).toBe('Garras');
    expect(text).toContain('Garras em Brisa');
    expect(text).toContain('Pedir salvaguarda de Brisa (CD 10, Constituição)');
    expect(text).toContain('Se falhar: Paralisado');
  });

  it('offers a grapple with its escape DC', async () => {
    const { groups } = await open();
    const text = (groups[1].textContent ?? '').replace(/\s+/g, ' ');
    expect(text).toContain('Agarrar (CD de fuga 12)');
    expect(text).toContain('Brisa fica agarrado');
  });

  it("reminds the master of the action's SRD text, where the exceptions the plan lacks are", async () => {
    const { groups } = await open();
    const srd = groups[0].querySelector('[data-testid="rider-srd"]');
    expect(srd?.textContent).toContain('Texto do SRD');
    expect(srd?.textContent).toContain('ignores elves and undead');
    expect(groups[0].querySelector('[data-testid="rider-size"]')).toBeNull();
  });

  it('says the size limit a grapple has, in Portuguese', async () => {
    const { groups } = await open();
    expect(groups[1].querySelector('[data-testid="rider-size"]')?.textContent).toContain(
      'Só criaturas até grande',
    );
  });

  it('applies with Aplicar and declines with Dispensar, each button named for its attack', async () => {
    const { fixture, groups, button } = await open();
    const apply = button(groups[0], 'Aplicar');
    expect(apply.getAttribute('aria-label')).toBe('Aplicar Garras');
    expect(apply.getAttribute('aria-describedby')).toBe('rider-r1-creature');
    apply.click();
    button(groups[1], 'Dispensar').click();
    expect(fixture.componentInstance.picks).toEqual([
      { id: 'r1', choice: HitRiderChoice.APPLY },
      { id: 'r2', choice: HitRiderChoice.DECLINE },
    ]);
  });

  it('reads without the ability when the stat block cannot be read, and never shows the monk buttons', async () => {
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({ providers: [creaturesProvider({})] });
    const fixture = TestBed.createComponent(Host);
    await settle(fixture);
    await settle(fixture);
    const text = ((fixture.nativeElement as HTMLElement).textContent ?? '').replace(/\s+/g, ' ');
    expect(text).toContain('Pedir salvaguarda de Brisa (CD 10)');
    expect(text).not.toContain('Golpe Atordoante');
  });
});
