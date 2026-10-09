import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import {
  ChoiceGroupSchema,
  ChoiceKind,
  ChoiceOptionSchema,
  ChoiceOrigin,
  ChoiceSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { ChoiceGroups, type ChoiceSelection } from './choice-groups';

function option(key: string, reasonPt = '') {
  return create(ChoiceOptionSchema, { key, storedKey: key, namePt: key.toUpperCase(), summaryPt: `s-${key}`, reasonPt });
}

function group(picks: number, picked: string[] = []) {
  return create(ChoiceGroupSchema, {
    origin: ChoiceOrigin.CLASS,
    sourceNamePt: 'Estilo de Luta',
    level: 1,
    choices: [
      create(ChoiceSchema, {
        key: 'c1',
        featureKey: 'f1',
        kind: ChoiceKind.OPTIONS,
        titlePt: 'Estilo de Luta',
        labelPt: 'Estilo de Luta',
        picks,
        picked,
        missing: Math.max(picks - picked.length, 0),
        options: [option('a'), option('b'), option('c', 'Pede nível 5')],
      }),
    ],
  });
}

describe('ChoiceGroups', () => {
  function setup(g = group(1)) {
    const fixture = TestBed.createComponent(ChoiceGroups);
    fixture.componentRef.setInput('groups', [g]);
    const sent: ChoiceSelection[] = [];
    fixture.componentInstance.selected.subscribe((s) => sent.push(s));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const cards = () => Array.from(el.querySelectorAll<HTMLElement>('.card'));
    return { fixture, el, cards, sent };
  }

  it('draws a one-pick choice as radios and reports the pick', () => {
    const { cards, sent, fixture } = setup();
    expect(cards().map((c) => c.getAttribute('role'))).toEqual(['radio', 'radio', 'radio']);
    cards()[1].click();
    fixture.detectChanges();
    expect(sent[0].optionKeys).toEqual(['b']);
    expect(cards()[1].getAttribute('aria-checked')).toBe('true');
  });

  it('keeps a blocked option focusable, disabled and described by its reason', () => {
    const { cards, sent } = setup();
    const blocked = cards()[2];
    expect(blocked.getAttribute('aria-disabled')).toBe('true');
    expect(blocked.getAttribute('tabindex')).not.toBeNull();
    expect(blocked.textContent).toContain('Pede nível 5');
    blocked.click();
    expect(sent.length).toBe(0);
  });

  it('blocks the unpicked options once a multiple choice is full', () => {
    const { cards, sent } = setup(group(2, ['a', 'b']));
    expect(cards()[0].getAttribute('role')).toBe('checkbox');
    expect(cards()[2].getAttribute('aria-disabled')).toBe('true');
    cards()[0].click();
    expect(sent[0].optionKeys).toEqual(['b']);
  });
});
