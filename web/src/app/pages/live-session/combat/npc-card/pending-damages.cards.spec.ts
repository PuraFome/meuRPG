import { TestBed } from '@angular/core/testing';

import { CombatantKind, PendingDamageStatus } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { PendingDamages } from './pending-damages';

/** What each card of the master's damage list says, for the shapes of pending damage the other specs do not reach. */
describe('PendingDamages: the words of a card', () => {
  const base = {
    id: 'p1',
    attackerId: 'gob',
    targetId: 'ana',
    status: PendingDamageStatus.ROLLED,
    diceCount: 2,
    diceSides: 6,
    bonus: 3,
    amount: 9,
  };

  let shown: ReturnType<typeof TestBed.createComponent<PendingDamages>>;

  function open(
    over: Record<string, unknown>,
    combatants: ReturnType<typeof combatant>[],
  ): HTMLElement {
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({ providers: [{ provide: CombatClient, useValue: {} }] });
    const fixture = TestBed.createComponent(PendingDamages);
    fixture.componentRef.setInput('pendings', [{ ...base, ...over } as never]);
    fixture.componentRef.setInput('encounter', encounter({ combatants } as never));
    fixture.componentRef.setInput('campaignId', 'camp');
    fixture.componentRef.setInput('state', new CombatState());
    fixture.detectChanges();
    shown = fixture;
    return fixture.nativeElement as HTMLElement;
  }
  const text = (el: HTMLElement) => (el.textContent ?? '').replace(/\s+/g, ' ').replace(/ /g, ' ');
  const typeIt = (el: HTMLElement) => {
    [...el.querySelectorAll('button')]
      .find((b) => (b.textContent ?? '').includes('Digitar o resultado'))!
      .click();
    shown.detectChanges();
  };
  const goblin = () => combatant({ id: 'gob', label: 'Goblin 1' });

  it('previews the hit points of a target after the amount after resistance, not the rolled one', () => {
    const el = open({ amountAfterSteps: 4 }, [
      goblin(),
      combatant({ id: 'ana', label: 'Ana', hitPointsCurrent: 20, hitPointsMax: 30 } as never),
    ]);
    expect(text(el)).toContain('Ana: 20 de 30 PV, depois 16');
    expect(text(el)).toContain('Aplicar 4 de dano');
  });

  it('counts the temporary hit points before the current ones', () => {
    const el = open({}, [
      goblin(),
      combatant({
        id: 'ana',
        label: 'Ana',
        hitPointsCurrent: 20,
        hitPointsMax: 30,
        hitPointsTemporary: 5,
      } as never),
    ]);
    expect(text(el)).toContain('Ana: 20 de 30 PV, depois 16');
  });

  it('shows no hit points line for a target that has none to show', () => {
    const el = open({}, [goblin(), combatant({ id: 'ana', label: 'Ana' } as never)]);
    expect(el.querySelector('.dmg__effect')).toBeNull();
    expect(text(el)).toContain('Aplicar 9 de dano');
  });

  it('falls back to the druid when the beast form has no hit points to read', () => {
    const el = open({}, [
      goblin(),
      combatant({
        id: 'ana',
        label: 'Ana',
        hitPointsCurrent: 20,
        hitPointsMax: 30,
        wildShapeBeastKey: 'monster:wolf',
      } as never),
    ]);
    expect(text(el)).toContain('Ana: 20 de 30 PV, depois 11');
  });

  it('names the beast "Forma animal" when it has no name, and passes nothing over when it holds', () => {
    const el = open({ amount: 4 }, [
      goblin(),
      combatant({
        id: 'ana',
        label: 'Ana',
        hitPointsCurrent: 20,
        hitPointsMax: 30,
        wildShapeBeastKey: 'monster:wolf',
        wildShapeBeastNamePt: '',
        wildShapeHitPointsCurrent: 11,
        wildShapeHitPointsMax: 11,
      } as never),
    ]);
    expect(text(el)).toContain('Forma animal: 11 de 11 PV, depois 7');
    expect(text(el)).not.toContain('o resto');
  });

  it('keeps the beast line alone when the druid has no hit points to read', () => {
    const el = open({ amount: 14 }, [
      goblin(),
      combatant({
        id: 'ana',
        label: 'Ana',
        wildShapeBeastKey: 'monster:wolf',
        wildShapeBeastNamePt: 'Lobo',
        wildShapeHitPointsCurrent: 11,
        wildShapeHitPointsMax: 11,
      } as never),
    ]);
    expect(text(el)).toContain('Lobo: 11 de 11 PV, depois 0');
    expect(text(el)).not.toContain('o resto');
  });

  it('says "Armadilha: dano" for a lone trap card with no names, with no number', () => {
    const el = open({ trapPointId: 'pt1', attackerId: '' }, [
      combatant({ id: 'ana', label: 'Ana', kind: CombatantKind.PLAYER }),
    ]);
    expect(el.querySelector('[data-testid="trap-part"]')?.textContent?.trim()).toBe(
      'Armadilha: dano',
    );
  });

  it('uses the trap and the damage type names when the server sends them', () => {
    const el = open(
      { trapPointId: 'pt1', attackerId: '', trapName: 'Dardos', damageTypePt: 'veneno' },
      [combatant({ id: 'ana', label: 'Ana', kind: CombatantKind.PLAYER })],
    );
    expect(el.querySelector('[data-testid="trap-part"]')?.textContent?.trim()).toBe(
      'Dardos: veneno',
    );
  });

  it('holds a hit that waits for a reaction with no window open, naming whoever reacts', () => {
    const el = open({ status: PendingDamageStatus.AWAITING_REACTION }, [
      goblin(),
      combatant({ id: 'ana', label: 'Ana' } as never),
    ]);
    expect(el.querySelector('.dmg__why')?.textContent?.trim()).toBe('blockEspere a reação da Ana.');
  });

  it('names a masculine target and falls back to "alvo" when the target is gone', () => {
    const he = open({ status: PendingDamageStatus.AWAITING_REACTION }, [
      goblin(),
      combatant({ id: 'ana', label: 'Toren' } as never),
    ]);
    expect(he.querySelector('.dmg__why')?.textContent).toContain('Espere a reação do Toren.');
    const gone = open({ status: PendingDamageStatus.AWAITING_REACTION }, [goblin()]);
    expect(gone.querySelector('.dmg__why')?.textContent).toContain('Espere a reação do alvo.');
  });

  it('waits for the player who rolls the damage of their own attack', () => {
    const el = open({ status: PendingDamageStatus.AWAITING_ROLL, attackerId: 'pen' }, [
      combatant({ id: 'pen', label: 'Pensantus', kind: CombatantKind.PLAYER }),
      combatant({ id: 'ana', label: 'Ana' } as never),
    ]);
    expect(text(el)).toContain('Esperando Pensantus rolar o dano em Ana.');
  });

  it('asks the master for the roll of an NPC attack, with the dice of the hit', () => {
    const el = open({ status: PendingDamageStatus.AWAITING_ROLL }, [
      goblin(),
      combatant({ id: 'ana', label: 'Ana' } as never),
    ]);
    typeIt(el);
    expect(text(el)).toContain('Role 2d6 para o dano: some os dois');
  });

  it('asks for one die in the singular', () => {
    const el = open({ status: PendingDamageStatus.AWAITING_ROLL, diceCount: 1 }, [
      goblin(),
      combatant({ id: 'ana', label: 'Ana' } as never),
    ]);
    typeIt(el);
    expect(text(el)).toContain('Role 1d6 para o dano');
    expect(text(el)).not.toContain('some os dois');
  });

  it('names the damage rolled in the formula line', () => {
    const el = open(
      {
        damageTypePt: 'cortante',
        roll: { diceCount: 2, diceSides: 6, faces: [3, 4], modifier: 3, total: 10 },
      },
      [goblin(), combatant({ id: 'ana', label: 'Ana' } as never)],
    );
    expect(el.querySelector('.dmg__formula')?.textContent).toContain('cortante');
  });

  it('uses the aria label with both names, empty when a combatant is gone', () => {
    const el = open({}, [goblin()]);
    expect(el.querySelector('.dmg')?.getAttribute('aria-label')).toBe('Dano de Goblin 1 em ');
  });
});
