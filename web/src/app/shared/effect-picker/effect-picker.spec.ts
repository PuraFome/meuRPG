import { ComponentFixture, TestBed } from '@angular/core/testing';

import { type EffectDraft, type EffectMenuVm, emptyEffect } from '../../core/content/effect-draft';
import {
  conditionMenu,
  menu,
  rangedAttackMenu,
  resistanceMenu,
} from '../../core/content/content-testing-menu';
import { costsMenu, grantedSpellMenu } from '../../core/content/content-testing-menu-costs';
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

  it("draws granted_spell's spell from the table's catalog, the ability in its own words, and the switches unfolded", () => {
    setup(
      { ...emptyEffect('granted_spell'), spell: 'spell:light', ability: 'choice' },
      { menu: grantedSpellMenu() },
    );
    fixture.componentRef.setInput('spellOptions', [
      { value: 'spell:light', label: 'Luz' },
      { value: 'spell:detect-magic', label: 'Detectar Magia' },
    ]);
    fixture.detectChanges();
    const spell = field('spell') as HTMLSelectElement;
    expect(Array.from(spell.options).map((o) => o.text.trim())).toEqual([
      'Escolha…',
      'Luz',
      'Detectar Magia',
    ]);
    expect(spell.value).toBe('spell:light');
    change(spell, 'Detectar Magia');
    expect(emitted.at(-1)?.spell).toBe('spell:detect-magic');

    const ability = field('ability') as HTMLSelectElement;
    expect(Array.from(ability.options).map((o) => o.text.trim())).toEqual([
      'Escolha…',
      'Inteligência',
      'Sabedoria',
      'Carisma',
      'O jogador escolhe',
    ]);
    expect(text(ability.closest('.frow')!)).toContain('Habilidade de conjuração');
    for (const sw of ['also_with_slot', 'ritual_only', 'counts_as_known']) {
      expect((field(sw) as HTMLInputElement).type).toBe('checkbox');
    }
    (field('ritual_only') as HTMLInputElement).dispatchEvent(new Event('change'));
    expect(emitted.at(-1)?.ritualOnly).toBe(true);
    // The resource each cast spends is folded until it holds something; empty means at will.
    expect(field('uses')).toBeNull();
    setup(
      {
        ...emptyEffect('granted_spell'),
        spell: 'spell:light',
        ability: 'cha',
        uses: 'usos_de_teste',
      },
      { menu: grantedSpellMenu() },
    );
    expect(text(field('uses')!.closest('.frow')!)).toContain('Vazio: à vontade, sem limite.');
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

describe('EffectPicker: resistance, condition immunity and the roll mode against a condition', () => {
  let fixture: ComponentFixture<EffectPicker>;
  let el: HTMLElement;
  const emitted: EffectDraft[] = [];

  function setup(effect: EffectDraft, m: EffectMenuVm) {
    emitted.length = 0;
    TestBed.resetTestingModule();
    fixture = TestBed.createComponent(EffectPicker);
    fixture.componentRef.setInput('effect', effect);
    fixture.componentRef.setInput('menu', m);
    fixture.componentRef.setInput('basePath', 'table_race.traits[0].effects[0]');
    fixture.componentRef.setInput('issuesOf', () => []);
    fixture.componentInstance.effectChange.subscribe((e) => emitted.push(e));
    fixture.detectChanges();
    el = fixture.nativeElement as HTMLElement;
  }
  const field = (path: string) =>
    el.querySelector<HTMLElement>(`[data-field="table_race.traits[0].effects[0].${path}"]`);

  it('asks the kind as three radio cards, shown from the start, with the resistance on when nothing is set', () => {
    setup(emptyEffect('resistance'), resistanceMenu());
    const radios = Array.from(el.querySelectorAll<HTMLInputElement>('input[type="radio"]'));
    expect(radios.map((r) => r.value)).toEqual(['resist', 'immune', 'vulnerable']);
    expect(radios.map((r) => r.checked)).toEqual([true, false, false]);
    expect(el.textContent).toContain('Não sofre dano.');
    expect(field('damage_types')).not.toBeNull();
    // The rarer fields stay folded: "Todos os tipos de dano, menos" and "Enquanto".
    expect(field('except')).toBeNull();
    expect(field('while')).toBeNull();
  });

  it('puts the card picked in the draft, and the damage types picked from the SRD ones plus the closed words', () => {
    setup(emptyEffect('resistance'), resistanceMenu());
    el.querySelectorAll<HTMLInputElement>('input[type="radio"]')[1].click();
    expect(emitted.at(-1)?.kind).toBe('immune');
    const select = field('damage_types')!.querySelector('select')!;
    expect(Array.from(select.options).map((o) => o.text.trim())).toEqual(
      expect.arrayContaining(['Todos os tipos de dano', 'Frio', 'Veneno']),
    );
    select.selectedIndex = Array.from(select.options).findIndex((o) => o.text.trim() === 'Frio');
    select.dispatchEvent(new Event('change'));
    expect(emitted.at(-1)?.damageTypes).toEqual(['damage-type:cold']);
  });

  it('lists the conditions to take none of, and the one a roll mode is against', () => {
    setup(emptyEffect('condition_immunity'), conditionMenu());
    const select = field('conditions')!.querySelector('select')!;
    expect(Array.from(select.options).map((o) => o.text.trim())).toEqual(
      expect.arrayContaining(['Amedrontado', 'Envenenado']),
    );
    setup(
      { ...emptyEffect('roll_mode'), roll: 'advantage', targets: ['save.wis'] },
      conditionMenu(),
    );
    expect(field('against_condition')).toBeNull(); // folded: optional
    el.querySelector<HTMLButtonElement>('button.more')!.click();
    fixture.detectChanges();
    expect(field('against_condition')).not.toBeNull();
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

describe("EffectPicker: costs, healing, hooks and recharge variants (the server's menu)", () => {
  const BASE = 'table_feat.effects[0]';
  const emitted: EffectDraft[] = [];
  let fixture: ComponentFixture<EffectPicker>;
  let el: HTMLElement;

  function setup(effect: EffectDraft, issues: Record<string, string[]> = {}) {
    emitted.length = 0;
    TestBed.resetTestingModule();
    fixture = TestBed.createComponent(EffectPicker);
    fixture.componentRef.setInput('effect', effect);
    fixture.componentRef.setInput('menu', costsMenu());
    fixture.componentRef.setInput('basePath', BASE);
    fixture.componentRef.setInput('featEditor', true);
    fixture.componentRef.setInput('issuesOf', (path: string) => issues[path] ?? []);
    fixture.componentInstance.effectChange.subscribe((e) => emitted.push(e));
    fixture.detectChanges();
    el = fixture.nativeElement as HTMLElement;
  }

  const text = (e: Element) => (e.textContent ?? '').replace(/\u00a0/g, ' ').replace(/\s+/g, ' ');
  const field = (name: string) => el.querySelector<HTMLElement>(`[data-field="${BASE}.${name}"]`);
  const input = (name: string) => field(name) as HTMLInputElement;
  const unfold = () => {
    (
      Array.from(el.querySelectorAll('button')).find((b) =>
        text(b).includes('Mais opções'),
      ) as HTMLButtonElement
    ).click();
    fixture.detectChanges();
  };
  const type = (name: string, value: string) => {
    input(name).value = value;
    input(name).dispatchEvent(new Event('input'));
  };

  it("draws the action's cost, maximum, heal, ability, temporary hit points and value per point from the menu, with their Portuguese labels", () => {
    setup({ ...emptyEffect('grant_action'), economy: 'bonus' });
    unfold();
    for (const name of ['cost', 'cost_max', 'heal', 'ability', 'temp_hp', 'value_per_point']) {
      expect(field(name), name).not.toBeNull();
    }
    expect(text(el)).toContain('Custo (usos)');
    expect(text(el)).toContain('Custo máximo (o jogador escolhe)');
    expect(text(el)).toContain('Cura (dados)');
    expect(text(el)).toContain('Pontos de vida temporários');
    expect(text(el)).toContain('Por ponto gasto a mais');
    expect(text(el)).toContain('Dados, como 1d6 ou 2d4+2');
  });

  it('writes each new field into the draft: text for the formulas and the dice, a choice for the ability', () => {
    setup({ ...emptyEffect('grant_action'), economy: 'bonus' });
    unfold();
    type('cost', '2');
    expect(emitted.at(-1)?.cost).toBe('2');
    type('cost_max', '3');
    expect(emitted.at(-1)?.costMax).toBe('3');
    type('heal', '1d6');
    expect(emitted.at(-1)?.heal).toBe('1d6');
    type('temp_hp', 'level()');
    expect(emitted.at(-1)?.tempHp).toBe('level()');
    type('value_per_point', '1d6');
    expect(emitted.at(-1)?.valuePerPoint).toBe('1d6');
  });

  it('opens a field that holds a value without the fold, and says what a recharge variant does', () => {
    setup({
      ...emptyEffect('resource'),
      resource: 'reserva_de_teste',
      max: '3',
      recharge: 'initiative',
    });
    expect(text(el)).toContain(
      'Volta um uso quando o personagem rola iniciativa, mas só se não restar nenhum.',
    );
    const select = field('recharge') as HTMLSelectElement;
    expect(Array.from(select.options).map((o) => o.text.trim())).toEqual([
      'Escolha…',
      'Descanso curto',
      'Descanso longo',
      'Ao rolar iniciativa',
      'Um uso por descanso curto',
      'Um uso por descanso longo',
    ]);
    setup({
      ...emptyEffect('resource'),
      resource: 'reserva_de_teste',
      max: '3',
      recharge: 'restore_one_on_long_rest',
    });
    expect(text(el)).toContain('um descanso curto não devolve nenhum.');
    setup({
      ...emptyEffect('resource'),
      resource: 'reserva_de_teste',
      max: '3',
      recharge: 'restore_one_on_short_rest',
    });
    expect(text(el)).toContain('um descanso longo devolve todos.');
  });

  it("picks the on_event event from the menu's events, required and open", () => {
    setup(emptyEffect('on_event'));
    const select = field('event') as HTMLSelectElement;
    expect(select).not.toBeNull();
    expect(Array.from(select.options).map((o) => o.text.trim())).toEqual([
      'Escolha…',
      'Ao terminar um descanso curto',
      'Ao terminar um descanso longo',
      'Ao falhar num teste contra a morte',
      'Ao cair a 0 pontos de vida',
    ]);
    select.selectedIndex = 4;
    select.dispatchEvent(new Event('change'));
    expect(emitted.at(-1)?.event).toBe('drop_to_zero');
  });

  it("draws the hook's result fields: the resource it spends, the one it gives back, the heal as a percentage of the maximum", () => {
    setup({ ...emptyEffect('on_event'), event: 'death_save_failed' });
    unfold();
    for (const name of [
      'resource',
      'cost',
      'heal',
      'ability',
      'heal_percent_max',
      'temp_hp',
      'restore',
      'value',
      'when',
      'text_pt',
    ]) {
      expect(field(name), name).not.toBeNull();
    }
    expect(text(el)).toContain('Gasta usos de');
    expect(text(el)).toContain('Devolve usos de');
    expect(text(el)).toContain('Cura (% do máximo)');
    type('heal_percent_max', '50');
    expect(emitted.at(-1)?.healPercentMax).toBe(50);
    type('restore', 'reserva_de_teste');
    expect(emitted.at(-1)?.restore).toBe('reserva_de_teste');
    type('heal_percent_max', '0');
    expect(emitted.at(-1)?.healPercentMax).toBe(0);
  });

  it('says the range of the percentage as the menu gives it (1 to 100)', () => {
    setup({ ...emptyEffect('on_event'), event: 'long_rest', healPercentMax: 50 });
    expect(text(field('heal_percent_max')!.closest('.frow')!)).toContain('De 1 a 100');
  });

  it("shows the server's refusal under the field it names, for every new path, and opens the folded field", () => {
    const paths = ['cost', 'cost_max', 'heal', 'ability', 'value_per_point', 'temp_hp', 'resource'];
    for (const name of paths) {
      setup(
        { ...emptyEffect('grant_action'), economy: 'bonus' },
        { [`${BASE}.${name}`]: [`Recusado em ${name}.`] },
      );
      const row = field(name)?.closest('.frow');
      expect(row, name).toBeTruthy();
      expect(text(row!)).toContain(`Recusado em ${name}.`);
      expect(row!.querySelector('.frow__hint')).toBeNull();
      // The refusal is the field's description, so a screen reader reads it with the field.
      expect(field(name)!.getAttribute('aria-describedby')).toBeTruthy();
    }
    for (const name of ['event', 'heal_percent_max', 'value', 'restore', 'resource']) {
      setup(emptyEffect('on_event'), { [`${BASE}.${name}`]: [`Recusado em ${name}.`] });
      const row = field(name)?.closest('.frow');
      expect(row, name).toBeTruthy();
      expect(text(row!)).toContain(`Recusado em ${name}.`);
    }
    setup(
      { ...emptyEffect('resource'), resource: 'reserva_de_teste', max: '2' },
      { [`${BASE}.recharge`]: ['Esse retorno não existe.'] },
    );
    expect(text(field('recharge')!.closest('.frow')!)).toContain('Esse retorno não existe.');
  });
});
