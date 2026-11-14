import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import { CombatantSchema } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  ActionEconomy,
  ActionOptionSchema,
  DisabledReasonCode,
  FeatureTargetWho,
  TurnOptionsSchema,
  UseScope,
} from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { ActionGroups } from './action-groups';

const plain = (t: string | null | undefined) =>
  (t ?? '').replace(/ /g, ' ').replace(/\s+/g, ' ').trim();

const entity = (over: object = {}) =>
  create(ActionOptionSchema, {
    enabled: true,
    viaUseFeature: true,
    useScope: UseScope.COMBAT,
    useTotal: 1,
    useLeft: 1,
    usesLeft: 2,
    resourceTotal: 2,
    action: {
      key: 'feature:symbiotic-entity',
      namePt: 'Entidade simbiótica',
      economy: ActionEconomy.ACTION,
      resourceKey: 'wild_shape',
    },
    ...over,
  } as never);

const halo = () =>
  create(ActionOptionSchema, {
    enabled: true,
    viaUseFeature: true,
    useScope: UseScope.COMBAT,
    useTotal: 2,
    useLeft: 1,
    targetMin: 1,
    targetMax: 1,
    targetWho: FeatureTargetWho.ENEMY,
    targetRangeFt: 10,
    action: { key: 'feature:halo', namePt: 'Halo de esporos', economy: ActionEconomy.REACTION },
  } as never);

function setup(features: ReturnType<typeof entity>[], locked = '') {
  const fixture = TestBed.createComponent(ActionGroups);
  fixture.componentRef.setInput('options', create(TurnOptionsSchema, { featureActions: features }));
  fixture.componentRef.setInput(
    'own',
    create(CombatantSchema, { movementLeftFt: 30, speedFt: 30 }),
  );
  fixture.componentRef.setInput('resources', [
    { key: 'wild_shape', namePt: 'Forma Selvagem', total: 2, used: 0, recharge: 'short_rest' },
  ]);
  fixture.componentRef.setInput('locked', locked);
  const used: string[] = [];
  fixture.componentInstance.feature.subscribe((k) => used.push(k));
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const row = (name: string) =>
    Array.from(el.querySelectorAll('app-action-row')).find(
      (r) => plain(r.querySelector('.row__name')?.textContent) === name,
    )!;
  return { el, row, used, fixture };
}

describe('ActionGroups: a feature taken with UseFeature (#743)', () => {
  it('says its scope, what is left and what it spends under the name, with "Usar"', () => {
    const { row } = setup([entity()]);
    const r = row('Entidade simbiótica');
    expect(plain(r.querySelector('.row__detail')?.textContent)).toBe(
      '1 por combate · restam 1 de 1 · gasta 1 uso de Forma Selvagem (restam 2 de 2)',
    );
    expect(plain(r.querySelector('button')?.textContent)).toBe('Usar');
  });

  it('presses "Usar" with the key of the action', () => {
    const { row, used, fixture } = setup([entity()]);
    (row('Entidade simbiótica').querySelector('button') as HTMLButtonElement).click();
    fixture.detectChanges();
    expect(used).toEqual(['feature:symbiotic-entity']);
  });

  it("writes the server's reason under an option that is off, and does not press it", () => {
    const { row, used, fixture } = setup([
      entity({
        enabled: false,
        useLeft: 0,
        reason: {
          code: DisabledReasonCode.NO_USES,
          textPt: 'Sem usos neste combate (1 por combate)',
        },
      }),
    ]);
    const r = row('Entidade simbiótica');
    expect(plain(r.querySelector('.row__why')?.textContent)).toContain(
      'Sem usos neste combate (1 por combate)',
    );
    const b = r.querySelector('button') as HTMLButtonElement;
    expect(b.getAttribute('aria-disabled')).toBe('true');
    b.click();
    fixture.detectChanges();
    expect(used).toEqual([]);
  });

  it('keeps a reaction without a button, with whom it takes and how far', () => {
    const { row } = setup([halo()]);
    const r = row('Halo de esporos');
    expect(plain(r.querySelector('.row__detail')?.textContent)).toBe(
      '2 por combate · restam 1 de 2 · 1 inimigo a até 3 m',
    );
    expect(r.querySelector('button')).toBeNull();
  });
});
