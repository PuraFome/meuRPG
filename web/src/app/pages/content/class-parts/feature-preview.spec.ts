import { ComponentFixture, TestBed } from '@angular/core/testing';

import { catalog } from '../../../core/content/content-testing';
import { menu } from '../../../core/content/content-testing-menu';
import { emptyEffect } from '../../../core/content/effect-draft';
import { emptyFeature, type FeatureDraft, withAsks } from '../../../core/content/feature-draft';
import { FeaturePreview } from './feature-preview';

describe('"Como os jogadores veem" for the open feature (#580, board 580-03)', () => {
  let fixture: ComponentFixture<FeaturePreview>;
  const option = (key: string, name: string, text: string): FeatureDraft => ({
    ...emptyFeature(),
    key,
    name,
    text,
  });
  const feature = (): FeatureDraft => ({
    ...emptyFeature(),
    name: 'Truques de lâmina',
    text: 'Escolha entre as opções da lista.',
    effects: [{ ...emptyEffect('choice'), choice: 'feature', count: 2 }],
    options: [
      option('feature:golpe@mesa', 'Golpe do Corvo', 'Soma o dado ao dano.'),
      withAsks(option('feature:asa@mesa', 'Asa Partida', 'Soma o dado ao ataque.'), {
        classKey: 'class:paladin',
        classLevel: '7',
        requiresOptions: ['feature:golpe@mesa'],
      }),
      option('', '', 'Sem nome não é salva.'),
    ],
  });
  const text = () => (fixture.nativeElement.textContent as string).replace(/\s+/g, ' ');

  function setFeature(f: FeatureDraft | null): void {
    fixture.componentRef.setInput('features', f ? [{ id: 'f1', level: 1, feature: f }] : []);
    fixture.componentRef.setInput('openId', f ? 'f1' : '');
  }

  function setup(f: FeatureDraft | null): void {
    TestBed.resetTestingModule();
    fixture = TestBed.createComponent(FeaturePreview);
    setFeature(f);
    fixture.componentRef.setInput('menu', menu());
    fixture.componentRef.setInput('catalog', catalog());
    fixture.detectChanges();
  }

  it('draws nothing while no feature is open', () => {
    setup(null);
    expect(fixture.nativeElement.querySelector('section')).toBeNull();
  });

  it('names the feature with "Da mesa", the choice, and each option with the "Pede" line the players read', () => {
    setup(feature());
    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('h2')?.textContent).toBe('Como os jogadores veem');
    expect(el.querySelector('.preview__name span')?.textContent).toBe('Truques de lâmina');
    expect(el.querySelector('.preview__name .mr-tag')?.textContent).toContain('Da mesa');
    expect(
      Array.from(el.querySelectorAll('dl div')).map((r) => [
        r.querySelector('dt')?.textContent,
        r.querySelector('dd')?.textContent,
      ]),
    ).toEqual([['Escolha', '2 de 2 opções']]);
    expect(text()).toContain('Golpe do Corvo. Soma o dado ao dano.');
    expect(text()).toContain(
      'Asa Partida. Soma o dado ao ataque. Pede Paladino 7 e Golpe do Corvo.',
    );
    // An option with no name is not sent, so players never read it.
    expect(text()).not.toContain('Sem nome não é salva');
    expect(el.querySelector('section')?.getAttribute('aria-labelledby')).toBe(
      el.querySelector('h2')?.id,
    );
  });

  it('follows the draft as it changes', () => {
    setup(feature());
    const f = feature();
    f.options[1] = withAsks(f.options[1], {
      classKey: 'class:paladin',
      classLevel: '9',
      requiresOptions: [],
    });
    setFeature(f);
    fixture.detectChanges();
    expect(text()).toContain('Pede Paladino 9.');
    expect(text()).not.toContain('Golpe do Corvo. Soma o dado ao dano. Pede');
  });

  it('calls a feature with no name "Nova característica"', () => {
    setup({ ...emptyFeature() });
    expect(text()).toContain('Nova característica');
  });
});
