import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import {
  CombatantEffectSchema,
  CombatantStateKind,
  ConditionSourceSchema,
  StatePhase,
} from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import { combatant } from '../../../../core/combat/combat-testing';
import { CombatantTags } from './combatant-tags';

const people = [combatant({ id: 'k', label: 'Kai' }), combatant({ id: 'b', label: 'Brisa' })];
const rage = create(CombatantEffectSchema, {
  id: 's1',
  kind: CombatantStateKind.RAGE,
  labelPt: 'Em fúria',
  effectPt: 'Vantagem em testes de Força e resistência a dano físico.',
  endsRound: 5,
});
const mark = create(CombatantEffectSchema, {
  id: 's2',
  kind: CombatantStateKind.HUNTERS_MARK_TARGET,
  labelPt: 'Marcado',
  effectPt: 'Dano extra de quem marcou.',
  sourceId: 'b',
  sourceLabel: 'Brisa',
  endsCombatantId: 'b',
  endsPhase: StatePhase.START_OF_TURN,
});

function setup(inputs: Record<string, unknown>) {
  const fixture = TestBed.createComponent(CombatantTags);
  fixture.componentRef.setInput('label', 'Toren');
  for (const [k, v] of Object.entries(inputs)) {
    fixture.componentRef.setInput(k, v);
  }
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement };
}

describe('CombatantTags: states and sources', () => {
  it('draws a state as a label with the word the server gave', () => {
    const { el } = setup({ states: [rage], people });
    const chip = el.querySelector<HTMLElement>('li.tag--state')!;
    expect(chip.querySelector('.tag__text')?.textContent?.trim()).toBe('Em fúria');
    expect(el.querySelector('button')).toBeNull();
    expect(el.querySelector('ul')?.getAttribute('aria-label')).toBe('Condições de Toren');
  });

  it('gives the label a text description a screen reader and a tooltip reach', () => {
    const { el } = setup({ states: [rage], people });
    const chip = el.querySelector<HTMLElement>('li.tag--state')!;
    const described = el.querySelector(`#${chip.getAttribute('aria-describedby')}`);
    expect(described?.textContent).toContain('Vantagem em testes de Força');
    expect(chip.getAttribute('title')).toContain('resistência a dano físico');
  });

  it('says who marked a target and until when', () => {
    const { el } = setup({ states: [mark], people });
    expect(el.querySelector('li.tag--state .tag__text')?.textContent?.trim()).toBe(
      'Marcado por Brisa',
    );
    expect(el.querySelector('.sr')?.textContent).toContain('Até o início do turno de Brisa');
  });

  it('says the round a rage ends in when it ends with no turn', () => {
    const { el } = setup({ states: [rage], people });
    expect(el.querySelector('.sr')?.textContent).toContain('Até a rodada 5');
  });

  it('adds the source and the end of a condition that comes from someone', () => {
    const { el } = setup({
      names: ['Atordoado', 'Cego'],
      people,
      sources: [
        create(ConditionSourceSchema, {
          conditionKey: 'condition:stunned',
          sourceLabel: 'Kai',
          endsCombatantId: 'k',
          endsPhase: StatePhase.END_OF_TURN,
        }),
      ],
    });
    const tags = Array.from(el.querySelectorAll('li')).map((li) =>
      li.textContent?.replace(/\s+/g, ' ').trim(),
    );
    expect(tags[0]).toBe('Atordoado , por Kai, até o fim do turno de Kai');
    expect(tags[1]).toBe('Cego');
  });

  it('keeps one tag and a count in the compact strip, states first', () => {
    const { el } = setup({ states: [rage], names: ['Cego'], people, compact: true });
    const items = Array.from(el.querySelectorAll('li')).map((li) =>
      (li.querySelector('.tag__text') ?? li).textContent?.trim(),
    );
    expect(items).toEqual(['Em fúria', '+1']);
  });

  it('draws nothing without a state or a condition', () => {
    const { el } = setup({});
    expect(el.querySelector('ul')).toBeNull();
  });
});
