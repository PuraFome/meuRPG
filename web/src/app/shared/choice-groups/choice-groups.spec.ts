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
import { applySelection } from './choice-picks';

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

  describe('a language or a tool the race or the background lets the player pick', () => {
    function pickGroup(origin: ChoiceOrigin, sourceNamePt: string, key: string, picks: number) {
      const stored = (k: string) => `${key}=${k}`;
      return create(ChoiceGroupSchema, {
        origin,
        sourceKey: 'race:half-elf',
        sourceNamePt,
        choices: [
          create(ChoiceSchema, {
            key,
            featureKey: 'race:half-elf',
            kind: ChoiceKind.OPTIONS,
            titlePt: picks === 1 ? 'Idioma' : 'Idiomas',
            labelPt: `${picks === 1 ? 'Idioma' : 'Idiomas'} (${sourceNamePt})`,
            picks,
            missing: picks,
            options: [
              create(ChoiceOptionSchema, {
                key: 'language:elvish',
                storedKey: stored('language:elvish'),
                namePt: 'Élfico',
                reasonPt: 'Você já fala este idioma.',
              }),
              create(ChoiceOptionSchema, {
                key: 'language:dwarvish',
                storedKey: stored('language:dwarvish'),
                namePt: 'Anão',
              }),
              create(ChoiceOptionSchema, {
                key: 'language:giant',
                storedKey: stored('language:giant'),
                namePt: 'Gigante',
              }),
            ],
          }),
        ],
      });
    }

    it('is a pick-one list under the race, with the language the character speaks blocked and told why', () => {
      const { el, cards, sent, fixture } = setup(
        pickGroup(ChoiceOrigin.RACE, 'Meio-elfo', 'race:half-elf#language', 1),
      );
      expect(el.querySelector('.choice__origin')?.textContent).toContain(
        'Raça · Meio-elfo · 1 escolha',
      );
      expect(cards().map((c) => c.getAttribute('role'))).toEqual(['radio', 'radio', 'radio']);
      const card = (name: string) =>
        cards().find((c) => (c.textContent ?? '').replace(/\s+/g, ' ').trim().startsWith(name))!;
      expect(card('Élfico').textContent).toContain('Você já fala este idioma.');
      expect(card('Élfico').getAttribute('aria-disabled')).toBe('true');
      card('Élfico').click();
      expect(sent).toEqual([]);

      card('Anão').click();
      fixture.detectChanges();
      expect(sent[0].optionKeys).toEqual(['language:dwarvish']);
      expect(applySelection([], sent[0].choice, sent[0].optionKeys)).toEqual([
        'race:half-elf#language=language:dwarvish',
      ]);
    });

    it("is a pick-many list under the background's name", () => {
      const { el, cards, sent } = setup(
        pickGroup(ChoiceOrigin.BACKGROUND, 'Acólito', 'background:acolyte#language', 2),
      );
      expect(el.querySelector('.choice__origin')?.textContent).toContain(
        'Antecedente · Acólito · 2 escolhas',
      );
      expect(cards().map((c) => c.getAttribute('role'))).toEqual([
        'checkbox',
        'checkbox',
        'checkbox',
      ]);
      cards()[1].click();
      expect(sent[0].optionKeys).toEqual(['language:dwarvish']);
    });
  });

  it('leads to the choice that stops the sheet before one that only warns', () => {
    const languages = create(ChoiceGroupSchema, {
      origin: ChoiceOrigin.BACKGROUND,
      sourceNamePt: 'Acólito',
      choices: [
        create(ChoiceSchema, {
          key: 'background:acolyte#language',
          kind: ChoiceKind.OPTIONS,
          titlePt: 'Idiomas',
          labelPt: 'Idiomas (Acólito)',
          picks: 2,
          missing: 2,
          advisory: true,
          options: [option('language:giant')],
        }),
      ],
    });
    const fixture = TestBed.createComponent(ChoiceGroups);
    fixture.componentRef.setInput('groups', [languages, group(1)]);
    fixture.detectChanges();
    const titles = Array.from(
      (fixture.nativeElement as HTMLElement).querySelectorAll<HTMLElement>('.choice__title'),
    );
    expect(fixture.componentInstance.focusFirstPending()).toBe(true);
    expect(document.activeElement).toBe(titles[1]);
    expect((titles[1].textContent ?? '').trim()).toContain('Estilo de Luta');
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
