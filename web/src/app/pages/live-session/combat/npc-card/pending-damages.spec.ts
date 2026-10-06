import { TestBed } from '@angular/core/testing';

import { CombatantKind, CriticalDamageRule, PendingDamageStatus, ReactionOutcome } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { PendingDamages } from './pending-damages';

describe('PendingDamages, a hit that waits for Escudo (E6-28b)', () => {
  const pending = { id: 'p1', attackerId: 'cap', targetId: 'pen', status: PendingDamageStatus.AWAITING_REACTION, diceCount: 1, diceSides: 6, bonus: 2 } as never;
  const enc = encounter({
    combatants: [combatant({ id: 'cap', label: 'Capitão Goblin' }), combatant({ id: 'pen', label: 'Pensantus', kind: CombatantKind.PLAYER })],
    reactionPrompts: [{ pendingDamageId: 'p1', targetId: 'pen', spellKey: 'spell:shield', spellNamePt: 'Escudo Arcano', slots: [{ level: 2, pact: false, free: 1 }, { level: 1, pact: false, free: 2 }, { level: 3, pact: false, free: 0 }] }],
  } as never);
  const api = { useReaction: vi.fn(), declineReaction: vi.fn() };

  function setup() {
    TestBed.configureTestingModule({ providers: [{ provide: CombatClient, useValue: api }] });
    const fixture = TestBed.createComponent(PendingDamages);
    fixture.componentRef.setInput('pendings', [pending]);
    fixture.componentRef.setInput('encounter', enc);
    fixture.componentRef.setInput('campaignId', 'camp');
    fixture.componentRef.setInput('state', new CombatState());
    const reacted: string[] = [];
    fixture.componentInstance.reacted.subscribe((r) => reacted.push(r));
    fixture.detectChanges();
    return { el: fixture.nativeElement as HTMLElement, reacted, fixture };
  }
  const button = (el: HTMLElement, name: string) => Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.includes(name))!;

  it('explains the wait, offers two same-size answers and disables "Rolar dano" with its reason', () => {
    const { el } = setup();
    expect(el.textContent).toContain('Esperando a reação do Pensantus.');
    expect(el.textContent).toContain('Ele pode conjurar Escudo Arcano (+5 na CA). O jogador decide sem ver o total; você pode responder por ele.');
    expect(button(el, 'Usar Escudo Arcano por ele').classList).toContain('dmg__skip');
    expect(button(el, 'Seguir sem Escudo Arcano').classList).toContain('dmg__skip');
    const off = button(el, 'Rolar dano');
    expect(off.getAttribute('aria-disabled')).toBe('true');
    expect(el.querySelector(`#${off.getAttribute('aria-describedby')}`)?.textContent).toContain('Espere a reação do Pensantus.');
  });

  it('casts Escudo for the target with the lowest free slot, and says what it did', async () => {
    api.useReaction.mockResolvedValue({ encounter: enc, outcome: ReactionOutcome.STOPPED });
    const { el, reacted, fixture } = setup();
    button(el, 'Usar Escudo Arcano por ele').click();
    await fixture.whenStable();
    expect(api.useReaction).toHaveBeenCalledWith('camp', 'enc', 'p1', { level: 1, pact: false }, expect.any(String));
    expect(reacted).toEqual(['stopped']);
  });

  it('lets the hit go with "Seguir sem Escudo"', async () => {
    api.declineReaction.mockResolvedValue(enc);
    const { el, reacted, fixture } = setup();
    button(el, 'Seguir sem Escudo Arcano').click();
    await fixture.whenStable();
    expect(api.declineReaction).toHaveBeenCalled();
    expect(reacted).toEqual(['declined']);
  });
});

describe('PendingDamages, the critical hint (RN-24)', () => {
  const enc = encounter({
    combatants: [combatant({ id: 'cap', label: 'Capitão Goblin' }), combatant({ id: 'pen', label: 'Pensantus', kind: CombatantKind.PLAYER })],
  } as never);

  function setup(over: Record<string, unknown>) {
    TestBed.configureTestingModule({ providers: [{ provide: CombatClient, useValue: {} }] });
    const fixture = TestBed.createComponent(PendingDamages);
    const pending = { id: 'p1', attackerId: 'cap', targetId: 'pen', status: PendingDamageStatus.AWAITING_ROLL, diceSides: 6, bonus: 2, ...over } as never;
    fixture.componentRef.setInput('pendings', [pending]);
    fixture.componentRef.setInput('encounter', enc);
    fixture.componentRef.setInput('campaignId', 'camp');
    fixture.componentRef.setInput('state', new CombatState());
    fixture.detectChanges();
    return fixture;
  }
  const hint = (el: HTMLElement) => (el.querySelector('[data-testid="critical-hint"]')?.textContent ?? '').replace(/\s+/g, ' ').trim();

  const typeIt = (fixture: ReturnType<typeof setup>) => {
    [...(fixture.nativeElement as HTMLElement).querySelectorAll('button')].find((b) => (b.textContent ?? '').includes('Digitar o resultado'))!.click();
    fixture.detectChanges();
  };
  const clean = (el: HTMLElement) => (el.textContent ?? '').replace(/\s+/g, ' ');

  it('says "role os dados duas vezes" for a critical hit under the SRD rule, once the master types a physical roll', () => {
    const fixture = setup({ diceCount: 4, critical: true, criticalRule: CriticalDamageRule.DOUBLED_DICE });
    const el = fixture.nativeElement as HTMLElement;
    // With the app's dice the server rolls them: no hint.
    expect(hint(el)).toBe('');
    typeIt(fixture);
    expect(hint(el)).toBe('Acerto crítico: role os dados duas vezes (4d6 no total).');
  });

  it('says "o máximo mais uma rolagem", with the maximum as the fixed part and the live total', () => {
    const fixture = setup({ diceCount: 2, critical: true, criticalRule: CriticalDamageRule.MAX_PLUS_ROLL, criticalMax: 12 });
    const el = fixture.nativeElement as HTMLElement;
    typeIt(fixture);
    expect(hint(el)).toBe('Acerto crítico: o máximo mais uma rolagem. O máximo dos dados (12) já vale sem rolar; role 2d6 uma vez.');
    // One instruction, the fixed parts named, and the live total the one the server records (typed 7 + 12 + 2 = 21).
    expect(clean(el)).toContain('Role 2d6 e digite só o que saiu, de 2 a 12. O app soma o resto.');
    expect(clean(el)).toContain('+ 12 do crítico + 2 de modificador');
    const field = el.querySelector('input.type__field, input[type="text"], input') as HTMLInputElement;
    field.value = '7';
    field.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(clean(el)).toContain('7 + 14 = 21');
  });

  it('has no hint for a hit that is not critical', () => {
    expect(hint(setup({ diceCount: 1 }).nativeElement)).toBe('');
  });
});
