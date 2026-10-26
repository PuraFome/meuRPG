import { ComponentFixture, TestBed } from '@angular/core/testing';

import { type EffectDraft, type EffectMenuVm, emptyEffect } from '../../core/content/effect-draft';
import {
  extraDamageMenu,
  menu,
  rangedAttackMenu,
  spellModifierMenu,
} from '../../core/content/content-testing-menu';
import { EffectPicker } from './effect-picker';

describe('EffectPicker (the closed menu of ADR-0018, from the server)', () => {
  const emitted: EffectDraft[] = [];
  let fixture: ComponentFixture<EffectPicker>;
  let el: HTMLElement;

  function setup(
    effect: EffectDraft,
    over: { issues?: Record<string, string[]>; allowTextOnly?: boolean; menu?: EffectMenuVm } = {},
  ) {
    emitted.length = 0;
    TestBed.resetTestingModule();
    fixture = TestBed.createComponent(EffectPicker);
    fixture.componentRef.setInput('effect', effect);
    fixture.componentRef.setInput('menu', over.menu ?? menu());
    fixture.componentRef.setInput('basePath', 'table_race.traits[0].effects[0]');
    fixture.componentRef.setInput('allowTextOnly', over.allowTextOnly ?? false);
    fixture.componentRef.setInput('issuesOf', (path: string) => over.issues?.[path] ?? []);
    fixture.componentInstance.effectChange.subscribe((e) => emitted.push(e));
    fixture.detectChanges();
    el = fixture.nativeElement as HTMLElement;
  }

  const text = (e: Element) => (e.textContent ?? '').replace(/\u00a0/g, ' ').replace(/\s+/g, ' ');
  const field = (path: string) =>
    el.querySelector<HTMLElement>(`[data-field="table_race.traits[0].effects[0].${path}"]`);
  const change = (select: HTMLSelectElement, label: string) => {
    select.selectedIndex = Array.from(select.options).findIndex((o) => o.text.trim() === label);
    select.dispatchEvent(new Event('change'));
  };

  it('lists the menu\'s types with "Só texto" first only where a feature may have none, and says what the type is for', () => {
    setup(emptyEffect('proficiency'), { allowTextOnly: true });
    const type = field('type') as HTMLSelectElement;
    expect(Array.from(type.options).map((o) => o.text.trim())).toEqual([
      'Só texto',
      'Modificador',
      'Proficiência',
      'Sentido',
      'Ação',
      'Dado de manobra',
      'Escolha',
      'Nota e magia concedida',
    ]);
    expect(text(el)).toContain('Dá proficiência.');
    setup(emptyEffect('proficiency'));
    expect(
      Array.from((field('type') as HTMLSelectElement).options).map((o) => o.text.trim()),
    ).not.toContain('Só texto');
  });

  it('draws the fields the menu says the type reads: the required one open, the optional ones folded under "Mais opções"', async () => {
    setup(emptyEffect('proficiency'));
    expect(field('proficiency')).not.toBeNull();
    expect(field('level')).toBeNull();
    expect(field('when')).toBeNull();
    expect(text(el)).toContain('Mais opções');
    (
      Array.from(el.querySelectorAll('button')).find((b) =>
        text(b).includes('Mais opções'),
      ) as HTMLButtonElement
    ).click();
    fixture.detectChanges();
    expect(field('level')).not.toBeNull();
    expect(field('when')).not.toBeNull();
    expect(text(el)).toContain('Menos opções');
  });

  it('opens an optional field that holds a value, and one a refusal points at, without being asked', () => {
    setup({ ...emptyEffect('proficiency'), proficiency: 'skill:perception', when: 'level() >= 5' });
    expect(field('when')).not.toBeNull();
    expect(field('level')).toBeNull();
    setup(emptyEffect('proficiency'), {
      issues: {
        'table_race.traits[0].effects[0].level': ['Este valor não serve para este efeito.'],
      },
    });
    expect(field('level')).not.toBeNull();
    expect(text(el)).toContain('Este valor não serve para este efeito.');
    expect(field('level')!.getAttribute('aria-invalid')).toBe('true');
  });

  it('takes the formula functions and the tag prefixes from the menu, not from the browser', () => {
    setup({ ...emptyEffect('modifier'), target: 'speed.walk', mode: 'add', tags: '' });
    expect(text(el)).toContain('Funções: mod("<habilidade>"), prof().');
    (
      Array.from(el.querySelectorAll('button')).find((b) =>
        text(b).includes('Mais opções'),
      ) as HTMLButtonElement
    ).click();
    fixture.detectChanges();
    expect(text(el)).toContain('começando por against:… ou about:….');
  });

  it('starts a new effect when the type changes: nothing of the old type is kept', () => {
    setup({ ...emptyEffect('proficiency'), proficiency: 'skill:perception' });
    change(field('type') as HTMLSelectElement, 'Sentido');
    expect(emitted).toHaveLength(1);
    expect(emitted[0].type).toBe('sense');
    expect(emitted[0].proficiency).toBe('');
  });

  it('offers the choices of a closed list from the menu, and a "choice" effect\'s `from` by its kind, which a new kind empties', () => {
    setup({ ...emptyEffect('choice'), choice: 'skill', count: 2, from: ['skill:arcana'] });
    const pick = el.querySelector('[data-field="table_race.traits[0].effects[0].from"]')!;
    expect(text(pick)).toContain('Arcanismo');
    const add = pick.querySelector('select') as HTMLSelectElement;
    expect(Array.from(add.options).map((o) => o.text.trim())).toEqual(['', 'Percepção']);
    add.selectedIndex = 1;
    add.dispatchEvent(new Event('change'));
    expect(emitted.at(-1)?.from).toEqual(['skill:arcana', 'skill:perception']);
    (pick.querySelector('.chip__x') as HTMLButtonElement).click();
    expect(emitted.at(-1)?.from).toEqual([]);
    change(field('choice') as HTMLSelectElement, 'Uma opção de uma lista do SRD');
    expect(emitted.at(-1)).toMatchObject({ choice: 'feature', from: [] });
  });

  it("asks a sense's range in metres and keeps feet underneath", () => {
    setup({ ...emptyEffect('sense'), sense: 'darkvision', rangeM: '18' });
    const range = field('range_ft') as HTMLInputElement;
    expect(range.value).toBe('18');
    range.value = '9';
    range.dispatchEvent(new Event('input'));
    expect(emitted.at(-1)?.rangeM).toBe('9');
  });

  it('offers "Só onde já tem proficiência" for the level Especialização on a skill, and turns it on', () => {
    setup({
      ...emptyEffect('proficiency'),
      proficiency: 'skill:perception',
      level: 'expertise',
    });
    const only = field('only_proficient') as HTMLInputElement;
    expect(only).not.toBeNull();
    expect(only.type).toBe('checkbox');
    expect(only.checked).toBe(false);
    expect(text(el)).toContain('Só onde já tem proficiência');
    expect(text(el)).toContain('Quem não é, fica como está.');
    only.dispatchEvent(new Event('change'));
    expect(emitted.at(-1)?.onlyProficient).toBe(true);

    // On, it reads as on and a refusal shows under it.
    setup(
      {
        ...emptyEffect('proficiency'),
        proficiency: 'skill:perception',
        level: 'expertise',
        onlyProficient: true,
      },
      {
        issues: {
          'table_race.traits[0].effects[0].only_proficient': [
            'Só vale para a especialização numa perícia.',
          ],
        },
      },
    );
    expect((field('only_proficient') as HTMLInputElement).checked).toBe(true);
    expect(text(el)).toContain('Só vale para a especialização numa perícia.');
  });

  it('leaves the switch out where it means nothing: another level, or a target that is not a skill', () => {
    setup({ ...emptyEffect('proficiency'), proficiency: 'skill:perception', level: 'full' });
    expect(field('only_proficient')).toBeNull();
    setup({ ...emptyEffect('proficiency'), proficiency: 'save.wis', level: 'expertise' });
    expect(field('only_proficient')).toBeNull();
    setup(emptyEffect('proficiency'));
    (
      Array.from(el.querySelectorAll('button')).find((b) =>
        text(b).includes('Mais opções'),
      ) as HTMLButtonElement
    ).click();
    fixture.detectChanges();
    expect(field('only_proficient')).toBeNull();
  });

  it("shows ranged_attack's switches and multiplier unfolded, though none is required, and turns a switch on", () => {
    const ranged = rangedAttackMenu();
    setup({ ...emptyEffect('ranged_attack'), applies: 'spell' }, { menu: ranged });
    const cover = field('ignore_cover') as HTMLInputElement;
    expect(cover).not.toBeNull();
    expect(cover.type).toBe('checkbox');
    expect(cover.checked).toBe(false);
    expect(field('range_multiplier')).not.toBeNull();
    expect(field('no_long_range_disadvantage')).not.toBeNull();
    expect(field('when')).toBeNull();
    expect(text(el)).toContain('Ignora meia cobertura e três quartos');
    expect(text(el)).toContain('A cobertura total continua impedindo o ataque.');
    cover.dispatchEvent(new Event('change'));
    expect(emitted.at(-1)?.ignoreCover).toBe(true);
    const times = field('range_multiplier') as HTMLInputElement;
    times.value = '2';
    times.dispatchEvent(new Event('input'));
    expect(emitted.at(-1)?.rangeMultiplier).toBe(2);

    // A switch that is on reads as on, and a refusal at it shows under it.
    setup(
      { ...emptyEffect('ranged_attack'), applies: 'weapon', noLongRangeDisadvantage: true },
      {
        menu: ranged,
        issues: {
          'table_race.traits[0].effects[0].no_long_range_disadvantage': ['Só para armas.'],
        },
      },
    );
    expect((field('no_long_range_disadvantage') as HTMLInputElement).checked).toBe(true);
    expect(text(field('no_long_range_disadvantage')!.closest('.frow')!)).toContain(
      'Só para armas.',
    );
  });

  it("shows extra_damage's dice, type, weapon and frequency unfolded, and sets them", () => {
    const m = extraDamageMenu();
    setup({ ...emptyEffect('extra_damage'), applies: 'any_target' }, { menu: m });
    for (const f of ['value', 'dice', 'dice_sides', 'damage_type', 'weapon', 'frequency']) {
      expect(field(f), f).not.toBeNull();
    }
    // The crit switch is rarer: folded until "Mais opções".
    expect(field('no_crit_doubling')).toBeNull();
    expect(text(el)).toContain('Dano fixo');
    expect(text(el)).toContain('Lados do dado');
    const dice = field('dice') as HTMLInputElement;
    dice.value = '1';
    dice.dispatchEvent(new Event('input'));
    expect(emitted.at(-1)?.dice).toBe('1');
    const sides = field('dice_sides') as HTMLInputElement;
    sides.value = '6';
    sides.dispatchEvent(new Event('input'));
    expect(emitted.at(-1)?.diceSides).toBe(6);
    change(field('damage_type') as HTMLSelectElement, 'Fogo');
    expect(emitted.at(-1)?.damageType).toBe('damage-type:fire');
    change(field('frequency') as HTMLSelectElement, 'Em todo acerto');
    expect(emitted.at(-1)?.frequency).toBe('each_hit');
    setup(
      {
        ...emptyEffect('extra_damage'),
        applies: 'any_target',
        dice: '1',
        diceSides: 6,
        noCritDoubling: true,
      },
      { menu: m },
    );
    expect((field('no_crit_doubling') as HTMLInputElement).checked).toBe(true);
  });

  it("shows a spell_modifier's fields by its change: the swap the types, the bonus its amount, the dice their count", () => {
    const m = spellModifierMenu();
    const base = { ...emptyEffect('spell_modifier'), appliesTo: 'damage_spell' };
    setup(base, { menu: m });
    expect(field('spell_change')).not.toBeNull();
    for (const f of ['from', 'value', 'count']) {
      expect(field(f), f).toBeNull();
    }
    setup({ ...base, spellChange: 'damage_type_swap' }, { menu: m });
    expect(text(el)).toContain('tipos de dano que troca');
    expect(el.querySelector('[data-field$=".from"]')).not.toBeNull();
    expect(field('value')).toBeNull();
    setup({ ...base, spellChange: 'bonus_damage' }, { menu: m });
    expect(field('value')).not.toBeNull();
    expect(field('damage_type')).not.toBeNull();
    expect(text(el)).toContain('Dano fixo somado');
    expect(field('count')).toBeNull();
    setup({ ...base, spellChange: 'extra_dice_per_slot_over' }, { menu: m });
    expect(text(el)).toContain('Dados a mais por nível acima');
    expect(field('count')).not.toBeNull();
    expect(field('value')).toBeNull();
  });

  it('puts the hint beside its field, and the refusal under it', () => {
    setup({ ...emptyEffect('modifier'), target: 'speed.walk', mode: 'add' });
    expect(el.querySelector('.frow__hint')).not.toBeNull();
    setup(
      { ...emptyEffect('modifier'), target: 'speed.walk', mode: 'add' },
      { issues: { 'table_race.traits[0].effects[0].value': ['Esta fórmula não funciona.'] } },
    );
    const row = field('value')!.closest('.frow')!;
    expect(text(row)).toContain('Esta fórmula não funciona.');
    expect(row.querySelector('.frow__hint')).toBeNull();
  });

  it('offers "Aumento de habilidade" only to the feat editor, with the abilities and amounts from the menu', () => {
    const types = () =>
      Array.from((field('type') as HTMLSelectElement).options).map((o) => o.text.trim());
    setup(emptyEffect('note'));
    expect(types()).not.toContain('Aumento de habilidade');
    TestBed.resetTestingModule();
    fixture = TestBed.createComponent(EffectPicker);
    fixture.componentRef.setInput('effect', emptyEffect('ability_increase'));
    fixture.componentRef.setInput('menu', menu());
    fixture.componentRef.setInput('basePath', 'table_race.traits[0].effects[0]');
    fixture.componentRef.setInput('featEditor', true);
    fixture.detectChanges();
    el = fixture.nativeElement as HTMLElement;
    expect(types()).toContain('Aumento de habilidade');
    expect(field('count')).not.toBeNull();
    const value = field('value') as HTMLSelectElement;
    expect(Array.from(value.options).map((o) => o.text.trim())).toEqual(['Escolha…', '+1', '+2']);
    const add = field('from')!.querySelector('select') as HTMLSelectElement;
    expect(Array.from(add.options).map((o) => o.text.trim())).toEqual([
      '',
      'Força',
      'Destreza',
      'Constituição',
    ]);
  });
});

describe('EffectPicker: the range field (metres on screen, feet on the wire)', () => {
  let fixture: ComponentFixture<EffectPicker>;
  let input: HTMLInputElement;

  function setup() {
    TestBed.resetTestingModule();
    fixture = TestBed.createComponent(EffectPicker);
    fixture.componentRef.setInput('effect', {
      ...emptyEffect('sense'),
      sense: 'darkvision',
      rangeM: '18',
    });
    fixture.componentRef.setInput('menu', menu());
    fixture.componentRef.setInput('basePath', 'table_race.traits[0].effects[0]');
    fixture.componentRef.setInput('allowTextOnly', false);
    fixture.componentRef.setInput('issuesOf', () => []);
    // The parent owns the draft: it takes what is emitted and passes it back, as the editor does.
    fixture.componentInstance.effectChange.subscribe((e: EffectDraft) => {
      fixture.componentRef.setInput('effect', e);
    });
    fixture.detectChanges();
    input = (fixture.nativeElement as HTMLElement).querySelector<HTMLInputElement>(
      '[data-field="table_race.traits[0].effects[0].range_ft"]',
    )!;
  }

  async function type(text: string) {
    input.value = text;
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  }

  it('keeps "4" as typed, though it is not a whole number of feet', async () => {
    setup();
    await type('4');
    expect(input.value).toBe('4');
  });

  it('keeps "4,5" as typed, character by character', async () => {
    setup();
    await type('4');
    expect(input.value).toBe('4');
    await type('4,');
    expect(input.value).toBe('4,');
    await type('4,5');
    expect(input.value).toBe('4,5');
  });
});
