import { create } from '@bufbuild/protobuf';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { type GetTrapNoticersResponse, GetTrapNoticersResponseSchema, MapPointKind, MapPointSchema, TrapNoticerSchema, TrapState } from '../../../../gen/meurpg/maps/v1/maps_pb';
import {
  Ability,
  ListTrapPresetsResponseSchema,
  TrapPassOutcome,
  TrapPresetSchema,
  TrapSaveApplies,
  TrapTargets,
  TrapTrigger,
} from '../../../../gen/meurpg/rules/v1/rules_pb';
import { MapsClient } from '../../../core/maps/maps-client';
import { FakeMapsClient } from '../../../core/maps/maps-testing';
import { TrapPresets } from '../../../core/traps/trap-presets';
import { TrapPointPanel } from './trap-point-panel';

const presets = create(ListTrapPresetsResponseSchema, {
  presets: [
    create(TrapPresetSchema, { key: 'trap:simple-pit', namePt: 'Fosso simples', descriptionPt: 'Um fosso.', noticeDc: 10, findDc: 10, trigger: TrapTrigger.ENTER, areaSize: 1, fallFt: 10, effect: { damage: [{ dice: '1d6', damageTypeKey: 'damage-type:bludgeoning', damageTypePt: 'concussão' }] } }),
    create(TrapPresetSchema, {
      key: 'trap:poison-needle',
      namePt: 'Agulha envenenada',
      descriptionPt: 'Uma agulha na fechadura.',
      findDc: 20,
      trigger: TrapTrigger.MANUAL,
      areaSize: 1,
      effect: {
        targets: TrapTargets.MANUAL,
        damage: [
          { dice: '1', damageTypeKey: 'damage-type:piercing', damageTypePt: 'perfurante' },
          { dice: '2d10', damageTypeKey: 'damage-type:poison', damageTypePt: 'veneno' },
        ],
        save: { ability: Ability.CONSTITUTION, dc: 15, appliesTo: TrapSaveApplies.CAUGHT, onFail: { condition: { conditionKey: 'condition:poisoned', durationPt: '1 hora' } }, onPass: TrapPassOutcome.NONE },
      },
    }),
  ],
  severities: [
    { key: 'setback', namePt: 'Revés', saveDcMin: 10, saveDcMax: 11, attackBonusMin: 3, attackBonusMax: 5 },
    { key: 'dangerous', namePt: 'Perigosa', saveDcMin: 12, saveDcMax: 15, attackBonusMin: 6, attackBonusMax: 8 },
    { key: 'deadly', namePt: 'Mortal', saveDcMin: 16, saveDcMax: 20, attackBonusMin: 9, attackBonusMax: 12 },
  ],
});

const trapPoint = create(MapPointSchema, {
  id: 'trap-1',
  mapId: 'map-1',
  kind: MapPointKind.TRAP,
  name: 'Fosso escondido',
  description: 'No corredor.',
  trap: { presetKey: 'trap:simple-pit', noticeDc: 15, findDc: 15, areaSize: 2, trigger: TrapTrigger.ENTER, state: TrapState.ARMED, effect: { damage: [{ dice: '2d6', damageTypeKey: 'damage-type:bludgeoning' }] } },
});

const noticers = create(GetTrapNoticersResponseSchema, {
  noticeDc: 15,
  noticers: [
    create(TrapNoticerSchema, { characterId: 'c1', characterName: 'Sálvia', passivePerception: 16, lightPenalty: 0, onMap: true, inRange: true, sees: true, wouldNotice: true, passesDc: true }),
    create(TrapNoticerSchema, { characterId: 'c2', characterName: 'Toren', passivePerception: 11, lightPenalty: -5, onMap: true, inRange: false, passesDc: false }),
  ],
});

describe('TrapPointPanel', () => {
  let fixture: ComponentFixture<TrapPointPanel>;
  let el: HTMLElement;
  let api: FakeMapsClient;

  async function setup(point = trapPoint, answer: GetTrapNoticersResponse | null = noticers) {
    api = new FakeMapsClient();
    api.noticers = answer;
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({
      providers: [
        { provide: MapsClient, useValue: api },
        { provide: TrapPresets, useValue: { list: () => Promise.resolve(presets) } },
      ],
    });
    fixture = TestBed.createComponent(TrapPointPanel);
    fixture.componentRef.setInput('point', point);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.detectChanges();
    el = fixture.nativeElement;
    await settle();
  }
  const settle = async () => {
    fixture.detectChanges();
    await new Promise((r) => setTimeout(r));
    await fixture.whenStable();
    fixture.detectChanges();
  };
  const text = () => (el.textContent ?? '').replace(/ /g, ' ').replace(/\s+/g, ' ');
  const button = (t: string) => Array.from(el.querySelectorAll<HTMLElement>('button')).find((b) => b.textContent?.trim().endsWith(t))!;
  const radio = (t: string) => Array.from(el.querySelectorAll<HTMLElement>('[role="radio"]')).find((b) => b.textContent?.trim().includes(t))!;
  const field = (label: string) => {
    const f = Array.from(el.querySelectorAll('mat-form-field')).find((x) => x.querySelector('mat-label')?.textContent?.trim() === label)!;
    return f.querySelector('input, textarea, select') as HTMLInputElement;
  };
  function type(label: string, value: string): void {
    const input = field(label);
    input.value = value;
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
  }
  const panel = () => fixture.componentInstance;

  it('lists the eight presets with a line each, and "Começar do zero"', async () => {
    await setup();
    expect(text()).toContain('Predefinições do SRD');
    expect(radio('Fosso simples').textContent).toContain('Queda de 3\u00a0m, 1d6');
    expect(radio('Agulha envenenada').textContent).toContain('1 perfurante, 2d10 veneno · resistência de Constituição');
    expect(button('Começar do zero')).toBeTruthy();
    expect(radio('Fosso simples').getAttribute('aria-checked')).toBe('true');
  });

  it('fills the form from a preset: the numbers, the trigger and the parts, all editable', async () => {
    await setup();
    radio('Agulha envenenada').click();
    await settle();
    expect(field('Nome').value).toBe('Agulha envenenada');
    expect(field('CD para notar (Percepção)').value).toBe('');
    expect(field('CD para achar (Investigação)').value).toBe('20');
    expect(text()).toContain('Dano que sempre acontece');
    expect(text()).toContain('Teste de resistência');
    expect(text()).toContain('CDs de resistência do SRD: revés 10 a 11 · perigosa 12 a 15 · mortal 16 a 20.');
    // The severity words judge nothing: there is no "A sua é perigosa".
    expect(text()).not.toMatch(/a sua,? .* é/i);
    expect(radio('Manual').getAttribute('aria-checked')).toBe('true');
    type('CD para achar (Investigação)', '17');
    expect(field('CD para achar (Investigação)').value).toBe('17');
  });

  it('says the image is visible to the players and keeps the DC labels of the ruling', async () => {
    await setup();
    expect(text()).toContain('Não desenhe a armadilha na imagem: os jogadores veem a imagem.');
    expect(text()).toContain('CD para notar (Percepção)');
    expect(text()).toContain('CD para achar (Investigação)');
  });

  it('adds and removes the parts of the effect as text actions, each part on its own', async () => {
    await setup(trapPoint);
    expect(el.querySelectorAll('[role="group"][aria-labelledby^="part-damage-"]')).toHaveLength(1);
    expect(text()).toContain('Acrescentar ao efeito');
    el.querySelector<HTMLElement>('button[aria-label="Acrescentar dano que sempre acontece"]')!.click();
    await settle();
    expect(el.querySelectorAll('[role="group"][aria-labelledby^="part-damage-"]')).toHaveLength(2);
    button('Ataque').click();
    el.querySelector<HTMLElement>('button[aria-label="Acrescentar teste de resistência"]')!.click();
    await settle();
    expect(el.querySelector('#part-attack')).not.toBeNull();
    expect(el.querySelector('#part-save')).not.toBeNull();
    expect(text()).toContain('Bônus de ataque do SRD: revés +3 a +5 · perigosa +6 a +8 · mortal +9 a +12.');
    el.querySelector<HTMLElement>('button[aria-label="Remover a parte Ataque"]')!.click();
    await settle();
    expect(el.querySelector('#part-attack')).toBeNull();
    el.querySelector<HTMLElement>('button[aria-label="Remover a parte Dano que sempre acontece 2"]')!.click();
    await settle();
    expect(el.querySelectorAll('[role="group"][aria-labelledby^="part-damage-"]')).toHaveLength(1);
  });

  it('is clean until something changes, then offers the whole spec', async () => {
    const dirty: boolean[] = [];
    await setup();
    panel().dirtyChange.subscribe((d) => dirty.push(d));
    expect(panel().changes()).toBeNull();
    type('CD para achar (Investigação)', '12');
    await settle();
    expect(dirty.at(-1)).toBe(true);
    const changes = panel().changes();
    expect(changes?.trap).toMatchObject({ findDc: 12, noticeDc: 15, areaSize: 2, presetKey: 'trap:simple-pit' });
    panel().discard();
    await settle();
    expect(field('CD para achar (Investigação)').value).toBe('15');
  });

  it('says what is wrong in the field, after the first try to save', async () => {
    await setup();
    type('CD para achar (Investigação)', '40');
    type('Nome', '   ');
    expect(text()).not.toContain('Use uma CD de 1 a 30.');
    expect(panel().changes()).toBeNull();
    await settle();
    expect(text()).toContain('Use uma CD de 1 a 30.');
    expect(text()).toContain('Dê um nome ao ponto.');
  });

  it('says a dice that is not a die', async () => {
    await setup();
    const dice = field('Dano');
    dice.value = '2d20';
    dice.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(panel().changes()).toBeNull();
    await settle();
    expect(text()).toContain('Use dados como 2d6');
  });

  it('shows "Quem notaria" from the server\'s numbers (the 9.14 table)', async () => {
    await setup();
    expect(api.calls).toContain('getTrapNoticers map-1 trap-1');
    expect(text()).toContain('Quem notaria');
    expect(text()).toContain('Sálvia');
    expect(text()).toContain('Se chegar a 3 m: não nota');
  });

  it('a trap with no DC to notice says nobody notices it alone', async () => {
    await setup(trapPoint, create(GetTrapNoticersResponseSchema, { noticeDc: 0, noticers: [] }));
    expect(text()).toContain('Ninguém nota esta armadilha sozinho');
  });

  it('has the state as three choices', async () => {
    await setup();
    expect(radio('Armada').getAttribute('aria-checked')).toBe('true');
    radio('Desarmada').click();
    await settle();
    expect(panel().changes()?.trap?.state).toBe(TrapState.DISARMED);
  });
});
