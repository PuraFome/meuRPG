import { type ComponentFixture, TestBed } from '@angular/core/testing';

import { type HitDraft, emptyHit } from '../../../core/content/hit-effect-draft';
import { HitEffectBlock } from './hit-effect-block';
import { settle } from '../../../../test-settle';

const text = (el: Element) => (el.textContent ?? '').replace(/ /g, ' ').replace(/\s+/g, ' ');
const ABILITIES = [
  { value: 'str', label: 'Força' },
  { value: 'con', label: 'Constituição' },
];
const DAMAGE = [
  { value: 'damage-type:fire', label: 'Fogo' },
  { value: 'damage-type:radiant', label: 'Radiante' },
];
const on = (over: Partial<HitDraft> = {}): HitDraft => ({
  ...emptyHit(),
  enabled: true,
  dice: '2',
  cap: '6',
  ...over,
});

describe('HitEffectBlock (#577)', () => {
  let fixture: ComponentFixture<HitEffectBlock>;
  const changed = vi.fn();

  function setup(inputs: Record<string, unknown> = {}) {
    changed.mockReset();
    TestBed.resetTestingModule();
    fixture = TestBed.createComponent(HitEffectBlock);
    const all: Record<string, unknown> = {
      hit: on(),
      target: 'creature',
      perSlot: 0,
      duration: 'timed',
      durationText: '1 minuto',
      concentration: true,
      abilities: ABILITIES,
      damageTypes: DAMAGE,
      pathOf: (f: string) => `table_spell.effect.modifiers[0].${f}`,
      ...inputs,
    };
    for (const [k, v] of Object.entries(all)) {
      fixture.componentRef.setInput(k, v);
    }
    fixture.componentInstance.hitChange.subscribe(changed);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }
  const field = (el: HTMLElement, f: string) =>
    el.querySelector<HTMLElement>(`[data-field="table_spell.effect.modifiers[0].${f}"]`)!;
  const cards = (el: HTMLElement) => Array.from(el.querySelectorAll<HTMLLabelElement>('.card'));
  const choose = (select: HTMLSelectElement, label: string) => {
    select.selectedIndex = Array.from(select.options).findIndex((o) => o.text.trim() === label);
    select.dispatchEvent(new Event('change'));
  };

  it('is off and explains why on an instantaneous spell', () => {
    const el = setup({ duration: 'instant', hit: on() });
    const sw = el.querySelector<HTMLButtonElement>('[role="switch"]')!;
    expect(sw.disabled).toBe(true);
    expect(sw.getAttribute('aria-checked')).toBe('false');
    const reason = el.querySelector('.lock')!;
    expect(text(reason)).toContain(
      'Uma magia instantânea não guarda efeito para o golpe. Escolha uma duração em Dados da magia.',
    );
    expect(sw.getAttribute('aria-describedby')).toBe(reason.id);
    expect(el.querySelector('.cards')).toBeNull();
  });

  it('is off by default on a spell that lasts, and turns on', async () => {
    const el = setup({ hit: emptyHit() });
    expect(text(el)).toContain('Desligado');
    expect(el.querySelector('.cards')).toBeNull();
    el.querySelector<HTMLButtonElement>('[role="switch"]')!.click();
    await settle(fixture, 2);
    expect(changed).toHaveBeenCalledWith(expect.objectContaining({ enabled: true }));
  });

  it('draws the four cards of "Quando vale" as one radio group, the checked one on', () => {
    const el = setup();
    const group = el.querySelector('[role="radiogroup"]')!;
    expect(group.getAttribute('aria-labelledby')).toBeTruthy();
    expect(
      cards(el).map(
        (c) =>
          `${text(c.querySelector('.title')!)}|${c.querySelector('.desc') ? text(c.querySelector('.desc')!) : ''}`,
      ),
    ).toEqual([
      'Próximo golpe com arma|Acerta uma vez e acaba.',
      'Próximo golpe corpo a corpo|À distância não gasta.',
      'Cada golpe enquanto durar|Todo golpe com arma.',
      'Cada golpe no alvo marcado|Só no alvo da magia.',
    ]);
    const radios = Array.from(el.querySelectorAll<HTMLInputElement>('.card input'));
    expect(radios.map((r) => r.checked)).toEqual([true, false, false, false]);
    expect(radios[0].getAttribute('data-field')).toBe('table_spell.effect.modifiers[0].trigger');
  });

  it('checks no card for a trigger that is unset, and says to choose', () => {
    const issues = (p: string) => (p.endsWith('.trigger') ? ['Escolha quando vale.'] : []);
    const el = setup({ hit: on({ trigger: '' }), issuesOf: issues });
    const radios = Array.from(el.querySelectorAll<HTMLInputElement>('.card input'));
    expect(radios.map((r) => r.checked)).toEqual([false, false, false, false]);
    expect(radios[0].getAttribute('data-field')).toBe('table_spell.effect.modifiers[0].trigger');
    expect(text(el.querySelector('.when')!)).toContain('Escolha quando vale.');
  });

  it('picks another trigger', async () => {
    const el = setup();
    el.querySelectorAll<HTMLInputElement>('.card input')[2].click();
    await settle(fixture, 2);
    expect(changed).toHaveBeenCalledWith(expect.objectContaining({ trigger: 'each_hit' }));
  });

  it.each([['area'], ['creatures'], ['self']])(
    'locks the marked target for the target %s, with the reason read by a screen reader',
    (target) => {
      const el = setup({ target });
      const marked = el.querySelectorAll<HTMLInputElement>('.card input')[3];
      expect(marked.disabled).toBe(true);
      const reason = el.querySelector(`#${marked.getAttribute('aria-describedby')}`)!;
      expect(text(reason)).toBe('lockPede o alvo Uma criatura.');
      expect(cards(el)[3].classList).toContain('card--locked');
    },
  );

  it('locks the marked target for extra targets per level, and opens it for one creature', () => {
    let el = setup({ perSlot: 1 });
    expect(el.querySelectorAll<HTMLInputElement>('.card input')[3].disabled).toBe(true);
    el = setup({ target: 'creature', perSlot: 0 });
    const marked = el.querySelectorAll<HTMLInputElement>('.card input')[3];
    expect(marked.disabled).toBe(false);
    expect(text(cards(el)[3])).not.toContain('Pede o alvo');
  });

  it('does not lock the marked target for a stale per-level count on a cantrip', () => {
    const el = setup({ perSlot: 2, level: 0 });
    expect(el.querySelectorAll<HTMLInputElement>('.card input')[3].disabled).toBe(false);
  });

  it('says in a polite live region that the lock moved the trigger', () => {
    const quiet = setup({ target: 'area', hit: on({ trigger: 'next_hit' }) });
    expect(quiet.querySelector('[role="status"]')).not.toBeNull();
    expect(text(quiet.querySelector('[role="status"]')!).trim()).toBe('');
    const el = setup({ target: 'area', moved: true, hit: on({ trigger: 'next_hit' }) });
    expect(text(el.querySelector('[role="status"]')!)).toContain(
      'Mudamos para “Próximo golpe com arma”: o alvo marcado pede Uma criatura.',
    );
  });

  it("says in the chips how long it lasts and that it is the caster's concentration, and where it lands", () => {
    let el = setup({ target: 'area' });
    expect(text(el.querySelector('.chips')!)).toContain('1 minuto');
    expect(text(el.querySelector('.chips')!)).toContain('Concentração · em quem conjura');
    expect(text(el.querySelector('.where')!)).toBe('em quem conjura, não na área');
    el = setup({ concentration: false });
    expect(text(el.querySelector('.chips')!)).not.toContain('Concentração');
    expect(text(el.querySelector('.where')!)).toBe('em quem conjura');
    el = setup({});
    expect(el.querySelector('.where')).toBeNull(); // the chip already says "em quem conjura"
  });

  it.each([['marked_target'], ['each_hit']])(
    'asks for concentration for %s only without it',
    (t) => {
      const trigger = t as HitDraft['trigger'];
      let el = setup({ concentration: false, hit: on({ trigger }) });
      expect(el.querySelector('.when .note')).not.toBeNull();
      el = setup({ concentration: true, hit: on({ trigger }) });
      expect(el.querySelector('.when .note')).toBeNull();
      el = setup({ concentration: false, hit: on({ trigger: 'next_hit' }) });
      expect(el.querySelector('.when .note')).toBeNull();
    },
  );

  it('warns, without blocking, that each hit asks for concentration', () => {
    const el = setup({ concentration: false, hit: on({ trigger: 'each_hit' }) });
    expect(text(el.querySelector('.note')!)).toContain('Ligue Concentração');
  });

  it('opens "Como funciona" with the explanation', async () => {
    const el = setup();
    const how = el.querySelector<HTMLElement>('.how')!;
    const button = el.querySelector<HTMLButtonElement>('.how-button')!;
    // The element the button controls is always there, hidden while closed.
    expect(button.getAttribute('aria-controls')).toBe(how.id);
    expect(how.hidden).toBe(true);
    expect(button.getAttribute('aria-expanded')).toBe('false');
    button.click();
    await settle(fixture, 2);
    expect(how.hidden).toBe(false);
    expect(button.getAttribute('aria-expanded')).toBe('true');
    expect(text(el.querySelector('.how')!)).toContain('Num crítico, os dados dobram.');
  });

  it('writes the damage fields into the draft', async () => {
    const el = setup();
    const dice = field(el, 'dice') as HTMLInputElement;
    dice.value = '3';
    dice.dispatchEvent(new Event('input'));
    await settle(fixture, 2);
    expect(changed).toHaveBeenLastCalledWith(expect.objectContaining({ dice: '3' }));
    const die = field(el, 'die') as HTMLSelectElement;
    expect(Array.from(die.options).map((o) => o.text.trim())).toEqual([
      'd4',
      'd6',
      'd8',
      'd10',
      'd12',
    ]);
    choose(die, 'd10');
    await settle(fixture, 2);
    expect(changed).toHaveBeenLastCalledWith(expect.objectContaining({ die: 10 }));
    const type = field(el, 'damage_type') as HTMLSelectElement;
    expect(type.options[0].text.trim()).toBe('O da arma');
    choose(type, 'Fogo');
    await settle(fixture, 2);
    expect(changed).toHaveBeenLastCalledWith(
      expect.objectContaining({ damageType: 'damage-type:fire' }),
    );
  });

  it('shows the ranges under the damage fields, and the cap is 1 to 20 with no "sem limite"', () => {
    const el = setup();
    const t = text(el);
    expect(t).toContain('1 a 10');
    expect(t).toContain('0 a 5 por nível acima');
    expect(t).toContain('1 a 20 dados.');
    expect(t).not.toContain('sem limite');
  });

  it('asks for a test before the condition and the duration', () => {
    const el = setup({ hit: on() });
    const cond = field(el, 'on_fail_conditions') as HTMLSelectElement;
    const lasts = field(el, 'condition_duration') as HTMLSelectElement;
    expect(cond.disabled).toBe(true);
    expect(lasts.disabled).toBe(true);
    expect(text(cond.options[0])).toBe('Pede um teste');
    expect(text(lasts.options[0])).toBe('Pede um teste');
    expect(text(el)).toContain('Contra a CD de magia de quem conjura.');
  });

  it('opens the condition and the duration with a save', async () => {
    const el = setup({ hit: on({ saveAbility: 'con' }) });
    const cond = field(el, 'on_fail_conditions') as HTMLSelectElement;
    const lasts = field(el, 'condition_duration') as HTMLSelectElement;
    expect(cond.disabled).toBe(false);
    expect(lasts.disabled).toBe(false);
    expect(text(el)).toContain('Contra a CD de magia de quem conjura.');
    expect(text(cond.options[0])).toBe('Escolha a condição');
    expect(Array.from(lasts.options).map((o) => o.text.trim())).toEqual([
      'Até o fim do próximo turno do alvo',
      'Até o fim da magia',
    ]);
    choose(cond, 'Impedido');
    await settle(fixture, 2);
    expect(changed).toHaveBeenLastCalledWith(
      expect.objectContaining({ condition: 'condition:restrained' }),
    );
    choose(lasts, 'Até o fim da magia');
    await settle(fixture, 2);
    expect(changed).toHaveBeenLastCalledWith(
      expect.objectContaining({ conditionLasts: 'spell_end' }),
    );
  });

  it('names the conditions as the glossary does', () => {
    const el = setup({ hit: on({ saveAbility: 'str' }) });
    const names = Array.from((field(el, 'on_fail_conditions') as HTMLSelectElement).options).map(
      (o) => o.text.trim(),
    );
    expect(names).toContain('Derrubado');
    expect(names).toContain('Impedido');
    expect(names).toContain('Cego');
  });

  it('clears the condition and the duration when the test is taken away', async () => {
    const el = setup({
      hit: on({ saveAbility: 'con', condition: 'condition:blinded', conditionLasts: 'spell_end' }),
    });
    choose(field(el, 'save') as HTMLSelectElement, 'Nenhum teste');
    await settle(fixture, 2);
    expect(changed).toHaveBeenLastCalledWith(
      expect.objectContaining({ saveAbility: '', condition: '', conditionLasts: 'next_turn' }),
    );
  });

  it('puts every refusal under its own field, with the range', async () => {
    const issues: Record<string, string[]> = {
      dice: ['De 1 a 10 dados.'],
      per_slot_dice: ['De 0 a 5 por nível.'],
      max_dice: ['De 1 a 20 dados.'],
      on_fail_conditions: ['Escolha a condição de quem falhar, ou tire o teste.'],
      trigger: ['Escolha quando vale.'],
    };
    // A refusal arrives after the save, never with the first render.
    const el = setup({ hit: on({ saveAbility: 'con' }) });
    fixture.componentRef.setInput(
      'issuesOf',
      (path: string) => issues[path.replace('table_spell.effect.modifiers[0].', '')] ?? [],
    );
    await settle(fixture, 3);
    for (const [f, msgs] of Object.entries(issues)) {
      const input = field(el, f);
      if (f !== 'trigger') {
        expect(input.getAttribute('aria-invalid'), f).toBe('true');
      }
      const described = input.getAttribute('aria-describedby');
      if (described) {
        expect(text(el.querySelector(`#${described}`)!)).toContain(msgs[0]);
      }
      expect(text(el)).toContain(msgs[0]);
    }
  });
});
