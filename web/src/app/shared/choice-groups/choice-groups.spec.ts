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
  return create(ChoiceOptionSchema, {
    key,
    storedKey: key,
    namePt: key.toUpperCase(),
    summaryPt: `s-${key}`,
    reasonPt,
  });
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

  describe('a feat the player chooses', () => {
    function featGroup(origin: ChoiceOrigin, sourceNamePt: string) {
      return create(ChoiceGroupSchema, {
        origin,
        sourceKey: 'race:humano-variante@mesa',
        sourceNamePt,
        choices: [
          create(ChoiceSchema, {
            key: 'trait:talento@mesa#feat',
            featureKey: 'trait:talento@mesa',
            kind: ChoiceKind.OPTIONS,
            titlePt: 'Talento',
            labelPt: 'Talento (Humano Variante)',
            picks: 1,
            missing: 1,
            options: [
              create(ChoiceOptionSchema, {
                key: 'feat:alerta@mesa',
                storedKey: 'trait:talento@mesa#feat=feat:alerta@mesa',
                namePt: 'Alerta',
                summaryPt: '+5 na Iniciativa.',
              }),
              create(ChoiceOptionSchema, {
                key: 'feat:lutador@mesa',
                storedKey: 'trait:talento@mesa#feat=feat:lutador@mesa',
                namePt: 'Lutador',
                summaryPt: 'Pede: Força 13.',
                reasonPt: 'Você ainda não cumpre o pré-requisito: Precisa de Força 13.',
              }),
            ],
          }),
        ],
      });
    }

    it('names the race that grants it and offers the feats with their text', () => {
      const { el, cards } = setup(featGroup(ChoiceOrigin.RACE, 'Humano Variante'));
      expect(el.querySelector('.choice__origin')?.textContent).toContain(
        'Raça · Humano Variante · 1 escolha',
      );
      expect(el.querySelector('.choice__title')?.textContent).toContain('Talento');
      expect(cards()[0].textContent).toContain('+5 na Iniciativa.');
    });

    it('shows the unmet prerequisite and does not let the feat be picked', () => {
      const { cards, sent } = setup(featGroup(ChoiceOrigin.BACKGROUND, 'Artesão'));
      expect(cards()[1].getAttribute('aria-disabled')).toBe('true');
      expect(cards()[1].textContent).toContain('Precisa de Força 13.');
      cards()[1].click();
      expect(sent.length).toBe(0);
    });

    it('says where a feat granted by a background or another feat comes from', () => {
      const bg = setup(featGroup(ChoiceOrigin.BACKGROUND, 'Artesão'));
      expect(bg.el.querySelector('.choice__origin')?.textContent).toContain(
        'Antecedente · Artesão',
      );
      const feat = setup(featGroup(ChoiceOrigin.FEAT, 'Mestre de Armas'));
      expect(feat.el.querySelector('.choice__origin')?.textContent).toContain(
        'Talento · Mestre de Armas',
      );
    });
  });
});
