import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { settle } from '../../../test-settle';
import {
  type EffectRow,
  type SpellEffectDraft,
  emptyEffect,
  emptyRow,
} from '../../core/content/spell-effect-draft';
import { type EffectContext, SpellEffect } from './spell-effect';

const CTX: EffectContext = {
  lasts: true,
  durationText: '1 minuto',
  concentration: true,
  hasSave: false,
  hasAttack: false,
  saveName: 'Sabedoria',
  self: false,
  targets: 'As 3 criaturas da magia.',
};

describe('SpellEffect (#579, boards 579-01, 579-02 and 579-04)', () => {
  function setup(
    effect: SpellEffectDraft,
    ctx: Partial<EffectContext> = {},
    issues: Record<string, string[]> = {},
  ) {
    TestBed.resetTestingModule();
    const fixture = TestBed.createComponent(SpellEffect);
    const sent: SpellEffectDraft[] = [];
    fixture.componentInstance.effectChange.subscribe((e) => sent.push(e));
    fixture.componentRef.setInput('effect', effect);
    fixture.componentRef.setInput('context', { ...CTX, ...ctx });
    fixture.componentRef.setInput('skills', [{ value: 'skill:stealth', label: 'Furtividade' }]);
    fixture.componentRef.setInput('damageTypes', [{ value: 'damage-type:fire', label: 'Fogo' }]);
    fixture.componentRef.setInput('abilityNames', [
      'Força',
      'Destreza',
      'Constituição',
      'Inteligência',
      'Sabedoria',
      'Carisma',
    ]);
    fixture.componentRef.setInput('issuesOf', (p: string) => issues[p] ?? []);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement, sent };
  }
  const text = (el: Element) => (el.textContent ?? '').replace(/ /g, ' ').replace(/\s+/g, ' ');
  const on = (p: Partial<SpellEffectDraft> = {}): SpellEffectDraft => ({
    ...emptyEffect(),
    on: true,
    ...p,
  });
  const field = (el: HTMLElement, path: string) =>
    el.querySelector<HTMLElement>(`[data-field="${path}"]`)!;

  it('is a switch, off: a spell that does not last says why it cannot be turned on', () => {
    const { el } = setup(emptyEffect(), { lasts: false });
    const sw = el.querySelector<HTMLButtonElement>('button[role="switch"]')!;
    expect(sw.disabled).toBe(true);
    expect(text(el)).toContain(
      'Uma magia instantânea não deixa efeito. Escolha uma duração em Dados da magia.',
    );
    expect(el.querySelector('.duration')).toBeNull();
    const lasting = setup(emptyEffect());
    expect(lasting.el.querySelector<HTMLButtonElement>('button[role="switch"]')!.disabled).toBe(
      false,
    );
  });

  it('turns on, with the duration and the concentration of the spell', async () => {
    const { fixture, el, sent } = setup(emptyEffect());
    el.querySelector<HTMLButtonElement>('button[role="switch"]')!.click();
    expect(sent.at(-1)?.on).toBe(true);
    fixture.componentRef.setInput('effect', sent.at(-1));
    await settle(fixture, 3);
    const t = text(el);
    expect(t).toContain('Dura 1 minuto');
    expect(t).toContain('Concentração');
    expect(t).toContain('Vêm da duração e da concentração da magia.');
    expect(t).toContain('Em quem fica');
    expect(t).toContain('As 3 criaturas da magia.');
  });

  it('keeps "Quem falhar no teste" and "Quem o ataque acertar" off while the spell has no save and no attack, and says why', () => {
    const { el } = setup(on());
    const radios = Array.from(
      el.querySelectorAll<HTMLInputElement>(
        'app-radio-cards[ng-reflect-label="Em quem fica"] input',
      ),
    );
    const inputs = Array.from(el.querySelectorAll<HTMLInputElement>('input[type="radio"]'));
    expect(radios.length >= 0 && inputs.length).toBeGreaterThan(0);
    const t = text(el);
    expect(t).toContain('Esta magia não tem teste.');
    expect(t).toContain('Esta magia não tem ataque.');
    const failed = inputs.find((i) =>
      i.closest('label')?.textContent?.includes('Quem falhar no teste'),
    )!;
    expect(failed.disabled).toBe(true);
    const saving = setup(on({ applies: 'failed_save' }), { hasSave: true });
    const f2 = Array.from(saving.el.querySelectorAll<HTMLInputElement>('input[type="radio"]')).find(
      (i) => i.closest('label')?.textContent?.includes('Quem falhar no teste'),
    )!;
    expect(f2.disabled).toBe(false);
    expect(f2.checked).toBe(true);
    expect(text(saving.el)).toContain('De Sabedoria, contra a sua CD.');
  });

  it('adds a change, picks its kind and sends the number as typed', async () => {
    const { fixture, el, sent } = setup(on());
    expect(text(el)).toContain('0 de 8');
    Array.from(el.querySelectorAll<HTMLButtonElement>('button'))
      .find((b) => text(b).includes('Adicionar o que muda'))!
      .click();
    fixture.componentRef.setInput('effect', sent.at(-1));
    await settle(fixture, 3);
    const kind = field(el, 'table_spell.effect.modifiers[0].kind') as HTMLSelectElement;
    kind.selectedIndex = Array.from(kind.options).findIndex((o) =>
      o.text.startsWith('PV temporários'),
    );
    kind.dispatchEvent(new Event('change'));
    fixture.componentRef.setInput('effect', sent.at(-1));
    await settle(fixture, 3);
    const value = field(el, 'table_spell.effect.modifiers[0].value') as HTMLInputElement;
    value.value = '10';
    value.dispatchEvent(new Event('input'));
    expect(sent.at(-1)?.rows[0]).toMatchObject({ kind: 'temp_hp', value: '10' });
    fixture.componentRef.setInput('effect', sent.at(-1));
    await settle(fixture, 3);
    expect(text(el)).toMatch(/No cartão:\s*PV temporários: 10/);
    expect(text(el)).toContain('1 de 8');
  });

  it('offers the seven kinds a table spell can leave', () => {
    const { el } = setup(on({ rows: [emptyRow()] }));
    const kind = field(el, 'table_spell.effect.modifiers[0].kind') as HTMLSelectElement;
    const names = Array.from(kind.options)
      .filter((o) => !o.disabled || o.value !== '')
      .map((o) => o.text.replace(/\s+/g, ' ').trim());
    expect(names.filter((n) => n && n !== 'Escolha…')).toHaveLength(7);
  });

  it('draws an aura row with the radius in metres, and keeps CA, deslocamento and PV temporários off with the reason', () => {
    const { el } = setup(on({ aura: { radiusM: '9', affects: 'allies' }, rows: [emptyRow()] }), {
      self: true,
    });
    expect((field(el, 'table_spell.effect.aura.radius_ft') as HTMLInputElement).value).toBe('9');
    const kind = field(el, 'table_spell.effect.modifiers[0].kind') as HTMLSelectElement;
    const off = Array.from(kind.options)
      .filter((o) => o.disabled && o.value !== '')
      .map((o) => o.value);
    expect(off.sort()).toEqual(['ac_bonus', 'speed', 'temp_hp']);
    expect(text(el)).toContain('não vale CA, deslocamento nem PV temporários, e não põe condição');
    expect(el.querySelector('app-search-pick')).toBeNull();
  });

  it('puts "Alcança" in its own group of three equal choices in one row (no wrap: a phone gets no orphan) under the range field', () => {
    const { el } = setup(on({ aura: { radiusM: '9', affects: 'allies' } }), { self: true });
    const group = el.querySelector<HTMLElement>('[aria-labelledby="spell-effect-reach"]')!;
    expect(group.querySelector('.group__label')?.textContent).toBe('Alcança');
    const labels = Array.from(group.querySelectorAll('label')).map((l) =>
      (l.textContent ?? '').replace('check', '').trim(),
    );
    expect(labels).toEqual(['Aliados', 'Inimigos', 'Só quem conjura']);
    expect(group.querySelector('app-segmented')?.classList.contains('seg--wrap')).toBe(false);
    expect(el.querySelector('.narrow app-text-field')).not.toBeNull();
    expect(text(el)).toContain('Contado a partir de quem conjura, no momento de cada jogada.');
  });

  it('keeps the aura off while the target is not the caster, and says so', () => {
    const { el } = setup(on());
    const sw = el.querySelectorAll<HTMLButtonElement>('button[role="switch"]')[1];
    expect(sw.disabled).toBe(true);
    expect(text(el)).toContain('Uma aura pede o alvo “Só quem conjura”.');
  });

  it('puts the refusal of a modifier on the row that writes it, even when a row writes more than one', async () => {
    const adv = { ...emptyRow('advantage'), rolls: ['attack', 'save'] as const, abilities: [] };
    const { fixture, el } = setup(
      on({ rows: [{ ...emptyRow('temp_hp'), value: '1500' }, adv as never] }),
      {},
      {
        'table_spell.effect.modifiers[0].value': ['Os PV temporários vão de 1 a 1.000.'],
        'table_spell.effect.modifiers[2].abilities': ['Escolha ao menos uma habilidade.'],
      },
    );
    await settle(fixture, 3);
    const first = field(el, 'table_spell.effect.modifiers[0].value');
    expect(el.querySelector(`#${first.getAttribute('aria-describedby')}`)?.textContent).toContain(
      'Os PV temporários vão de 1 a 1.000.',
    );
    const abilities = field(el, 'table_spell.effect.modifiers[2].abilities');
    expect(abilities).toBeTruthy();
    expect(el.querySelector('[data-field="table_spell.effect.modifiers[1].abilities"]')).toBeNull();
    expect(text(el)).toContain('Escolha ao menos uma habilidade.');
    expect(text(el)).toContain('Os PV temporários vão de 1 a 1.000.');
    expect(text(el)).toContain('Escolha ao menos uma habilidade.');
  });

  it('shows a modifier the menu does not offer as it is, and says what to use instead', () => {
    const { el } = setup(on({ rows: [{ ...emptyRow('unknown') }] }));
    expect(text(el)).toContain('Este tipo não vale numa magia da mesa.');
    expect(text(el)).toContain('use uma condição (Incapacitado, Paralisado)');
  });

  it("asks the players' label only for an effect the whole table sees, and counts to 30", async () => {
    const { fixture, el } = setup(on({ label: 'Abençoado' }));
    const label = field(el, 'table_spell.effect.player_label_pt') as HTMLInputElement;
    expect(label.value).toBe('Abençoado');
    expect(label.getAttribute('maxlength')).toBe('30');
    expect(text(el)).toContain('9 de 30');
    fixture.componentRef.setInput('effect', on({ visibility: 'owner' }));
    await settle(fixture, 3);
    expect(el.querySelector('[data-field="table_spell.effect.player_label_pt"]')).toBeNull();
  });

  describe('as the block of a feature aura (#579)', () => {
    const aura = (rows: EffectRow[] = [{ ...emptyRow('roll'), rollMode: 'ability' }]) =>
      on({ aura: { radiusM: '3', affects: 'allies' }, rows });
    function feature(effect: SpellEffectDraft) {
      const out = setup(effect);
      out.fixture.componentRef.setInput('feature', true);
      out.fixture.componentRef.setInput('basePath', 'table_feat.effects[0].aura');
      out.fixture.detectChanges();
      return out;
    }

    it('has no switch of its own and says it holds the whole combat', () => {
      const { el } = feature(aura());
      expect(el.querySelector('button[role="switch"]')).toBeNull();
      expect(text(el)).toContain(
        'Vale em todo combate, enquanto quem leva a aura estiver de pé e consciente.',
      );
      expect(text(el)).not.toContain('Em volta de quem conjura');
    });

    it('takes the number of a roll from an ability, with a least, and writes both', () => {
      const { el, sent } = feature(aura());
      const ability = field(
        el,
        'table_feat.effects[0].aura.modifiers[0].value_ability',
      ) as HTMLSelectElement;
      expect(ability).toBeTruthy();
      expect(ability.value).toBe('cha');
      const least = field(
        el,
        'table_feat.effects[0].aura.modifiers[0].value_min',
      ) as HTMLInputElement;
      expect(least.value).toBe('1');
      least.value = '2';
      least.dispatchEvent(new Event('input'));
      expect(sent.at(-1)!.rows[0]).toMatchObject({ rollMode: 'ability', valueMin: '2' });
    });

    it('offers the ability only in a feature, not in a spell', () => {
      const spell = setup(on({ rows: [{ ...emptyRow('roll') }] }));
      expect(text(spell.el)).not.toContain('De uma habilidade');
      const own = feature(aura([{ ...emptyRow('roll') }]));
      expect(text(own.el)).toContain('De uma habilidade');
    });
  });
});
