import { TestBed } from '@angular/core/testing';

import { NpcCheckAskState } from '../../../../gen/meurpg/play/v1/combat_pb';
import { cardOf } from '../../../core/npc-check/npc-check-card';
import { ask, npcCheckRead } from '../../../core/npc-check/npc-check-testing';
import { NpcCheckCard } from './npc-check-card';

function mount(read = npcCheckRead(), busy = false, error = '') {
  const fixture = TestBed.createComponent(NpcCheckCard);
  fixture.componentRef.setInput('card', cardOf(read));
  fixture.componentRef.setInput('busy', busy);
  fixture.componentRef.setInput('error', error);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  return { fixture, el, text: () => (el.textContent ?? '').replace(/\s+/g, ' ') };
}

const buttons = (el: HTMLElement) => Array.from(el.querySelectorAll('button'));
const named = (el: HTMLElement, words: string) => buttons(el).find((b) => (b.textContent ?? '').includes(words) || b.getAttribute('aria-label') === words);

describe('NpcCheckCard', () => {
  it('waits for the bards: the creature and the check, the total, who is asked, and the only way out is to go on', () => {
    const { el, text } = mount();
    expect(el.querySelector('h3')?.textContent).toBe('Capitão bandido · Enganação');
    expect(text()).toContain('d20 (13) + 4 = 17');
    expect(text()).toContain('Antes das Palavras de Interrupção: o total fecha quando os bardos responderem.');
    expect(text()).toContain('Esperando Brisa');
    expect(text()).toContain('Na fila');
    expect(text()).toContain('Só você vê');
    expect(named(el, 'Continuar sem esperar')).toBeTruthy();
    expect(named(el, 'Esconder o teste')).toBeUndefined();
  });

  it('asks to go on without waiting, and not while it is on the way', () => {
    const { fixture, el } = mount();
    let skipped = 0;
    fixture.componentInstance.skip.subscribe(() => skipped++);
    named(el, 'Continuar sem esperar')?.click();
    expect(skipped).toBe(1);
    const busy = mount(npcCheckRead(), true);
    expect(named(busy.el, 'Continuar sem esperar')?.getAttribute('aria-disabled')).toBe('true');
  });

  it('settled: the X hides it, nothing waits, and it says who cut and who did not answer', () => {
    const read = npcCheckRead({
      settled: true,
      total: 13,
      asks: [ask('Brisa', NpcCheckAskState.CUT, { face: 4 }), ask('bilbo', NpcCheckAskState.CLOSED, { sides: 6 })],
    });
    const { fixture, el, text } = mount(read);
    expect(text()).toContain('17 − 4 = 13');
    expect(text()).toContain('Cortou 4');
    expect(text()).toContain('Sem resposta');
    expect(text()).toContain('Fica aqui até o próximo teste ou a sessão acabar.');
    expect(named(el, 'Continuar sem esperar')).toBeUndefined();
    let hidden = 0;
    fixture.componentInstance.dismiss.subscribe(() => hidden++);
    named(el, 'Esconder o teste')?.click();
    expect(hidden).toBe(1);
  });

  it('says a creature with no number for the check rolled +0', () => {
    const { el } = mount(npcCheckRead({ modifierKnown: false, settled: true, asks: [] }));
    expect(el.querySelector('[data-testid="no-bonus"]')?.textContent).toContain(
      'Sem bônus: Capitão bandido não tem número para Enganação. O d20 contou com +0: some o bônus que achar justo.',
    );
    expect(mount().el.querySelector('[data-testid="no-bonus"]')).toBeNull();
  });

  it('says nobody was asked when the check settled at once', () => {
    const { el } = mount(npcCheckRead({ settled: true, asks: [] }));
    expect(el.querySelector('[data-testid="nobody"]')?.textContent).toContain('Ninguém foi perguntado');
  });

  it('shows the error of a skip that did not go through', () => {
    const { el } = mount(npcCheckRead(), false, 'Não deu para continuar sem esperar agora. Tente de novo.');
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('Não deu para continuar sem esperar agora.');
  });

  it('never reads "Sem efeito"', () => {
    expect(mount(npcCheckRead({ settled: true })).text()).not.toContain('Sem efeito');
  });
});
