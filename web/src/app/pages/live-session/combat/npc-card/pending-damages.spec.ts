import { TestBed } from '@angular/core/testing';

import {
  CombatantKind,
  CriticalDamageRule,
  PendingDamageStatus,
  ReactionKind,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter, reactionWindow } from '../../../../core/combat/combat-testing';
import { PendingDamages } from './pending-damages';

describe('PendingDamages, a hit that waits for a reaction (PM-04)', () => {
  const pending = (status: PendingDamageStatus) =>
    ({
      id: 'p1',
      attackerId: 'cap',
      targetId: 'pen',
      status,
      diceCount: 1,
      diceSides: 6,
      bonus: 2,
    }) as never;
  const combatants = [
    combatant({ id: 'cap', label: 'Capitão Goblin' }),
    combatant({ id: 'pen', label: 'Pensantus', kind: CombatantKind.PLAYER }),
    combatant({ id: 'm1', label: 'Mago 1' }),
  ];

  function setup(status: PendingDamageStatus, windows: ReturnType<typeof reactionWindow>[]) {
    TestBed.configureTestingModule({ providers: [{ provide: CombatClient, useValue: {} }] });
    const fixture = TestBed.createComponent(PendingDamages);
    fixture.componentRef.setInput('pendings', [pending(status)]);
    fixture.componentRef.setInput(
      'encounter',
      encounter({ combatants, reactionWindows: windows } as never),
    );
    fixture.componentRef.setInput('campaignId', 'camp');
    fixture.componentRef.setInput('state', new CombatState());
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }
  const buttons = (el: HTMLElement) =>
    Array.from(el.querySelectorAll('button')).map((b) => b.textContent?.trim());

  it('turns "Rolar dano" grey and dashed with the reason written, and has no answer of its own', () => {
    const el = setup(PendingDamageStatus.AWAITING_REACTION, [
      reactionWindow({ id: 'w1', reactorId: 'm1', reactorLabel: 'Mago 1' }),
    ]);
    const off = Array.from(el.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('Rolar dano'),
    )!;
    expect(off.getAttribute('aria-disabled')).toBe('true');
    expect(
      el
        .querySelector(`#${off.getAttribute('aria-describedby')}`)
        ?.textContent?.replace(/\u00a0/g, ' '),
    ).toContain('Espere a reação do Mago 1.');
    // The answer is the master's card (the queue), not this box.
    expect(buttons(el).some((b) => b?.includes('Usar'))).toBe(false);
  });

  it('holds a damage still to roll while any window is open, and says to answer the request above for the check', () => {
    const el = setup(PendingDamageStatus.AWAITING_ROLL, [
      reactionWindow({ id: 'w1', kind: ReactionKind.MASTER_CHECK }),
    ]);
    expect(el.textContent).toContain('Responda ao pedido acima.');
    expect(el.querySelector('app-roll-picker')).toBeNull();
  });

  it('offers the roll again once no window is open', () => {
    const el = setup(PendingDamageStatus.AWAITING_ROLL, []);
    expect(el.querySelector('app-roll-picker')).not.toBeNull();
  });
});

describe('PendingDamages, the critical hint (RN-24)', () => {
  const enc = encounter({
    combatants: [
      combatant({ id: 'cap', label: 'Capitão Goblin' }),
      combatant({ id: 'pen', label: 'Pensantus', kind: CombatantKind.PLAYER }),
    ],
  } as never);

  function setup(over: Record<string, unknown>) {
    TestBed.configureTestingModule({ providers: [{ provide: CombatClient, useValue: {} }] });
    const fixture = TestBed.createComponent(PendingDamages);
    const pending = {
      id: 'p1',
      attackerId: 'cap',
      targetId: 'pen',
      status: PendingDamageStatus.AWAITING_ROLL,
      diceSides: 6,
      bonus: 2,
      ...over,
    } as never;
    fixture.componentRef.setInput('pendings', [pending]);
    fixture.componentRef.setInput('encounter', enc);
    fixture.componentRef.setInput('campaignId', 'camp');
    fixture.componentRef.setInput('state', new CombatState());
    fixture.detectChanges();
    return fixture;
  }
  const hint = (el: HTMLElement) =>
    (el.querySelector('[data-testid="critical-hint"]')?.textContent ?? '')
      .replace(/\s+/g, ' ')
      .trim();

  const typeIt = (fixture: ReturnType<typeof setup>) => {
    [...(fixture.nativeElement as HTMLElement).querySelectorAll('button')]
      .find((b) => (b.textContent ?? '').includes('Digitar o resultado'))!
      .click();
    fixture.detectChanges();
  };
  const clean = (el: HTMLElement) => (el.textContent ?? '').replace(/\s+/g, ' ');

  it('says "role os dados duas vezes" for a critical hit under the SRD rule, once the master types a physical roll', () => {
    const fixture = setup({
      diceCount: 4,
      critical: true,
      criticalRule: CriticalDamageRule.DOUBLED_DICE,
    });
    const el = fixture.nativeElement as HTMLElement;
    // With the app's dice the server rolls them: no hint.
    expect(hint(el)).toBe('');
    typeIt(fixture);
    expect(hint(el)).toBe('Acerto crítico: role os dados duas vezes (4d6 no total).');
  });

  it('says "o máximo mais uma rolagem", with the maximum as the fixed part and the live total', () => {
    const fixture = setup({
      diceCount: 2,
      critical: true,
      criticalRule: CriticalDamageRule.MAX_PLUS_ROLL,
      criticalMax: 12,
    });
    const el = fixture.nativeElement as HTMLElement;
    typeIt(fixture);
    expect(hint(el)).toBe(
      'Acerto crítico: o máximo mais uma rolagem. O máximo dos dados (12) já vale sem rolar; role 2d6 uma vez.',
    );
    // One instruction, the fixed parts named, and the live total the one the server records (typed 7 + 12 + 2 = 21).
    expect(clean(el)).toContain('Role 2d6 e digite só o que saiu, de 2 a 12. O app soma o resto.');
    expect(clean(el)).toContain('+ 12 do crítico + 2 de modificador');
    const field = el.querySelector(
      'input.type__field, input[type="text"], input',
    ) as HTMLInputElement;
    field.value = '7';
    field.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(clean(el)).toContain('7 + 14 = 21');
  });

  it('has no hint for a hit that is not critical', () => {
    expect(hint(setup({ diceCount: 1 }).nativeElement)).toBe('');
  });
});
