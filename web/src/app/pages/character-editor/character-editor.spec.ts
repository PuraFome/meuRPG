import { Injectable } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FormBuilder } from '@angular/forms';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';
import { of } from 'rxjs';

import {
  CharacterBlockedReason,
  CharacterBlockedSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { CharacterEditor } from './character-editor';
import { createAttackGroup } from './npc-short-form/basic-form';
import {
  CharacterEditorSource,
  CharacterForEdit,
  CreateCharacterInput,
  RulesCatalogVm,
  SpellDetailsVm,
  UpdateCharacterInput,
} from './character-editor.types';

@Injectable()
class FakeCharacterEditorSource {
  loadCatalogFn: (campaignId: string) => Promise<RulesCatalogVm> = () => Promise.resolve(catalog());
  loadCharacterForEditFn: (campaignId: string, characterId: string) => Promise<CharacterForEdit> =
    () => Promise.reject(new Error('not stubbed'));
  createCharacterCalls: CreateCharacterInput[] = [];
  createCharacterFn: (input: CreateCharacterInput) => Promise<{ characterId: string }> = () =>
    Promise.resolve({ characterId: 'new-char' });
  updateCharacterCalls: UpdateCharacterInput[] = [];
  updateCharacterFn: (input: UpdateCharacterInput) => Promise<{ revision: number }> = () =>
    Promise.resolve({ revision: 2 });

  loadCatalog(campaignId: string): Promise<RulesCatalogVm> {
    return this.loadCatalogFn(campaignId);
  }
  loadSpellDetailsCalls: string[] = [];
  loadSpellDetailsFn: (spellKey: string) => Promise<SpellDetailsVm> = (key) =>
    Promise.resolve(knockDetails(key));
  loadSpellDetails(_campaignId: string, spellKey: string): Promise<SpellDetailsVm> {
    this.loadSpellDetailsCalls.push(spellKey);
    return this.loadSpellDetailsFn(spellKey);
  }
  loadCharacterForEdit(campaignId: string, characterId: string): Promise<CharacterForEdit> {
    return this.loadCharacterForEditFn(campaignId, characterId);
  }
  createCharacter(input: CreateCharacterInput): Promise<{ characterId: string }> {
    this.createCharacterCalls.push(input);
    return this.createCharacterFn(input);
  }
  updateCharacter(input: UpdateCharacterInput): Promise<{ revision: number }> {
    this.updateCharacterCalls.push(input);
    return this.updateCharacterFn(input);
  }
}

function knockDetails(key: string): SpellDetailsVm {
  return {
    key,
    namePt: 'Arrombar',
    nameEn: 'Knock',
    level: 2,
    schoolNamePt: 'Transmutação',
    ritual: false,
    concentration: false,
    castingTime: { amount: 1, unit: 'action', trigger: '', raw: '1 action' },
    range: { kind: 'ranged', distanceFt: 60, raw: '60 feet' },
    components: { verbal: true, somatic: false, material: false, materialText: '' },
    duration: {
      kind: 'instantaneous',
      amount: 0,
      unit: '',
      upTo: false,
      concentration: false,
      raw: 'Instantaneous',
    },
    description: ['Choose an object that you can see within range.'],
    higherLevel: [],
  };
}

function catalog(): RulesCatalogVm {
  return {
    races: [
      {
        key: 'race:gnome',
        namePt: 'Gnomo',
        constitutionBonus: 0,
        subraces: [{ key: 'subrace:rock-gnome', namePt: 'Gnomo da Rocha', constitutionBonus: 1 }],
      },
    ],
    classes: [
      {
        key: 'class:wizard',
        namePt: 'Mago',
        hitDie: 6,
        isCaster: true,
        preparation: 'spellbook',
        subclasses: [{ key: 'subclass:evocation', namePt: 'Evocação' }],
        subclassLevel: 2,
        spellcastingFirstLevel: 1,
        // Levels 1-5: circles 1, 1, 2, 2, 3.
        maxSpellLevelByLevel: [1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 9, 9],
      },
      {
        key: 'class:paladin',
        namePt: 'Paladino',
        hitDie: 10,
        isCaster: true,
        preparation: 'prepared',
        subclasses: [{ key: 'subclass:devotion', namePt: 'Devoção' }],
        subclassLevel: 3,
        spellcastingFirstLevel: 2,
        // No leveled spells at level 1.
        maxSpellLevelByLevel: [0, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5],
      },
    ],
    backgrounds: [{ key: 'background:acolyte', namePt: 'Acólito' }],
    skills: [
      { key: 'skill:arcana', namePt: 'Arcanismo', ability: 'int' },
      { key: 'skill:history', namePt: 'História', ability: 'int' },
    ],
    armor: [{ key: 'equipment:leather-armor', namePt: 'Armadura de Couro' }],
    weapons: [
      { key: 'equipment:quarterstaff', namePt: 'Bordão' },
      { key: 'equipment:dagger', namePt: 'Adaga' },
    ],
    spells: [
      { key: 'spell:fire-bolt', namePt: 'Raio de Fogo', level: 0, classKeys: ['class:wizard'] },
      { key: 'spell:ray-of-frost', namePt: 'Raio de Gelo', level: 0, classKeys: ['class:wizard'] },
      {
        key: 'spell:magic-missile',
        namePt: 'Mísseis Mágicos',
        level: 1,
        classKeys: ['class:wizard'],
      },
      { key: 'spell:shield', namePt: 'Escudo Arcano', level: 1, classKeys: ['class:wizard'] },
      { key: 'spell:fireball', namePt: 'Bola de Fogo', level: 3, classKeys: ['class:wizard'] },
      { key: 'spell:bless', namePt: 'Bênção', level: 1, classKeys: ['class:paladin'] },
      // Not on the Wizard's list — proves the picker filters by class.
      {
        key: 'spell:cure-wounds',
        namePt: 'Curar Ferimentos',
        level: 1,
        classKeys: ['class:cleric'],
      },
    ],
  };
}

function routeParams(params: Record<string, string>) {
  return { paramMap: of(convertToParamMap(params)) };
}

function flush(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

describe('CharacterEditor', () => {
  let fake: FakeCharacterEditorSource;

  function configure(params: Record<string, string>): void {
    TestBed.configureTestingModule({
      imports: [CharacterEditor],
      providers: [
        provideRouter([]),
        { provide: CharacterEditorSource, useClass: FakeCharacterEditorSource },
        { provide: ActivatedRoute, useValue: routeParams(params) },
      ],
    });
    fake = TestBed.inject(CharacterEditorSource) as unknown as FakeCharacterEditorSource;
  }

  async function render(): Promise<{
    fixture: ComponentFixture<CharacterEditor>;
    el: HTMLElement;
  }> {
    const fixture = TestBed.createComponent(CharacterEditor);
    fixture.detectChanges();
    await flush();
    await fixture.whenStable();
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  it('builds the create-character request from the form and the selected skills', async () => {
    configure({ id: 'camp-1' });
    const { fixture } = await render();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const cmp = fixture.componentInstance as any;

    cmp.fullForm.patchValue({
      name: 'Pensantus',
      race: 'race:gnome',
      subrace: 'subrace:rock-gnome',
      className: 'class:wizard',
      subclassName: 'subclass:evocation',
      level: 3,
      background: 'background:acolyte',
      equipmentText: 'Grimório\nAdaga',
    });
    cmp.selectedSkills.set(new Set(['skill:arcana', 'skill:history']));
    cmp.selectedCantrips.set(new Set(['spell:fire-bolt', 'spell:ray-of-frost']));

    await cmp.submit();

    expect(fake.createCharacterCalls.length).toBe(1);
    const req = fake.createCharacterCalls[0];
    expect(req.campaignId).toBe('camp-1');
    expect(req.kind).toBe('player');
    expect(req.full?.name).toBe('Pensantus');
    expect(req.full?.race).toBe('race:gnome');
    expect(req.full?.level).toBe(3);
    expect(req.full?.skillProficiencies.sort()).toEqual(['skill:arcana', 'skill:history']);
    // Content keys, never the typed name — the whole point of the picker.
    expect(req.full?.cantrips.sort()).toEqual(['spell:fire-bolt', 'spell:ray-of-frost']);
    expect(req.full?.equipmentText).toBe('Grimório\nAdaga');
    expect(req.basic).toBeNull();
  });

  it('enforces ability scores between 1 and 30', async () => {
    configure({ id: 'camp-1' });
    const { fixture } = await render();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const cmp = fixture.componentInstance as any;

    cmp.fullForm.get('abilities.str')?.setValue(31);
    expect(cmp.fullForm.get('abilities.str')?.invalid).toBe(true);

    cmp.fullForm.get('abilities.str')?.setValue(0);
    expect(cmp.fullForm.get('abilities.str')?.invalid).toBe(true);

    cmp.fullForm.get('abilities.str')?.setValue(18);
    expect(cmp.fullForm.get('abilities.str')?.invalid).toBe(false);
  });

  it("shows only the spell lists that match the class's preparation style", async () => {
    configure({ id: 'camp-1' });
    const { fixture, el } = await render();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const cmp = fixture.componentInstance as any;

    // The fixture's only class is a "spellbook" caster (Wizard): both
    // "Magias conhecidas" and "Magias preparadas" show, alongside "Truques".
    cmp.fullForm.patchValue({ className: 'class:wizard' });
    fixture.detectChanges();

    expect(el.textContent).toContain('Truques');
    expect(el.textContent).toContain('Magias conhecidas');
    expect(el.textContent).toContain('Magias preparadas');
  });

  it('never lets a person type a content key — no free-text input for spells, weapons or armor', async () => {
    configure({ id: 'camp-1' });
    const { fixture, el } = await render();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const cmp = fixture.componentInstance as any;

    cmp.fullForm.patchValue({ className: 'class:wizard' });
    fixture.detectChanges();

    // Every remaining <textarea> is one of the genuinely free-text fields;
    // none carries a content-key control name.
    const textareas = Array.from(el.querySelectorAll('textarea')).map((t) =>
      t.getAttribute('formcontrolname'),
    );
    expect(textareas).not.toContain('cantripsText');
    expect(textareas).not.toContain('spellsKnownText');
    expect(textareas).not.toContain('spellsPreparedText');
    expect(textareas).not.toContain('weaponsText');
    expect(textareas.sort()).toEqual(
      ['customFeaturesText', 'equipmentText', 'languagesText', 'toolProficienciesText'].sort(),
    );
    // Armor is a select, not a free-text input.
    expect(el.querySelector('input[formcontrolname="armor"]')).toBeNull();
  });

  it("filters cantrips and spells to the chosen class's list", async () => {
    configure({ id: 'camp-1' });
    const { fixture } = await render();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const cmp = fixture.componentInstance as any;

    cmp.fullForm.patchValue({ className: 'class:wizard' });

    const cantripKeys = cmp.availableCantrips().map((s: { key: string }) => s.key);
    expect(cantripKeys.sort()).toEqual(['spell:fire-bolt', 'spell:ray-of-frost']);

    const spellKeys = cmp.availableSpells().map((s: { key: string }) => s.key);
    // The Cleric-only spell never shows for a Wizard.
    expect(spellKeys.sort()).toEqual(['spell:fireball', 'spell:magic-missile', 'spell:shield']);
  });

  it('the search box narrows the spell picker by Portuguese name', async () => {
    configure({ id: 'camp-1' });
    const { fixture } = await render();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const cmp = fixture.componentInstance as any;

    cmp.fullForm.patchValue({ className: 'class:wizard' });
    cmp.cantripsFilter.set('gelo');

    expect(cmp.filteredCantrips().map((s: { key: string }) => s.key)).toEqual([
      'spell:ray-of-frost',
    ]);
  });

  describe('the subclass', () => {
    it('offers "Nenhuma" and saves the subclass unset when it is picked', async () => {
      configure({ id: 'camp-1' });
      const { fixture } = await render();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cmp = fixture.componentInstance as any;

      cmp.fullForm.patchValue({
        name: 'Pensantus',
        race: 'race:gnome',
        className: 'class:wizard',
        subclassName: 'subclass:evocation',
        level: 2,
        background: 'background:acolyte',
      });
      // "Nenhuma" is the option whose value is ''.
      cmp.fullForm.patchValue({ subclassName: '' });
      await cmp.submit();

      expect(fake.createCharacterCalls[0].full?.subclassName).toBe('');
    });

    it('lists "Nenhuma" first in the select', async () => {
      configure({ id: 'camp-1' });
      const { fixture, el } = await render();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cmp = fixture.componentInstance as any;
      cmp.fullForm.patchValue({ className: 'class:wizard' });
      fixture.detectChanges();

      const select = el.querySelector<HTMLElement>('mat-select[formcontrolname="subclassName"]');
      select?.querySelector<HTMLElement>('.mat-mdc-select-trigger')?.click();
      fixture.detectChanges();
      const options = Array.from(document.querySelectorAll('mat-option')).map((o) =>
        o.textContent?.trim(),
      );
      expect(options[0]).toBe('Nenhuma');
      expect(options).toContain('Evocação');
    });

    it('says at which level the class chooses it, until the level gets there', async () => {
      configure({ id: 'camp-1' });
      const { fixture, el } = await render();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cmp = fixture.componentInstance as any;

      cmp.fullForm.patchValue({ className: 'class:paladin', level: 1 });
      fixture.detectChanges();
      expect(el.textContent).toContain('O Paladino escolhe a subclasse no nível 3.');

      cmp.fullForm.patchValue({ level: 3 });
      fixture.detectChanges();
      expect(el.textContent).not.toContain('escolhe a subclasse no nível');
    });

    it('clears the subclass when the class changes', async () => {
      configure({ id: 'camp-1' });
      const { fixture } = await render();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cmp = fixture.componentInstance as any;

      cmp.fullForm.patchValue({
        className: 'class:wizard',
        subclassName: 'subclass:evocation',
        customSubclassName: 'Outra',
      });
      cmp.fullForm.patchValue({ className: 'class:paladin' });
      cmp.onClassChange();

      expect(cmp.fullForm.value.subclassName).toBe('');
      expect(cmp.fullForm.value.customSubclassName).toBe('');
    });
  });

  describe('the spell lists by level', () => {
    const keys = (list: { key: string }[]) => list.map((s) => s.key);

    it('lists only the spells up to the highest circle of the level, circle then name', async () => {
      configure({ id: 'camp-1' });
      const { fixture } = await render();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cmp = fixture.componentInstance as any;

      cmp.fullForm.patchValue({ className: 'class:wizard', level: 1 });
      expect(keys(cmp.filteredSpellsKnown())).toEqual(['spell:shield', 'spell:magic-missile']);
      // Cantrips are not gated by level, and sort by name.
      expect(keys(cmp.filteredCantrips())).toEqual(['spell:fire-bolt', 'spell:ray-of-frost']);

      cmp.fullForm.patchValue({ level: 5 });
      expect(keys(cmp.filteredSpellsPrepared())).toEqual([
        'spell:shield',
        'spell:magic-missile',
        'spell:fireball',
      ]);
    });

    it('keeps a selected spell above the limit, last and marked, so it can be unchecked', async () => {
      configure({ id: 'camp-1' });
      const { fixture, el } = await render();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cmp = fixture.componentInstance as any;

      cmp.fullForm.patchValue({ className: 'class:wizard', level: 5 });
      cmp.selectedSpellsKnown.set(new Set(['spell:fireball']));
      cmp.fullForm.patchValue({ level: 1 });
      fixture.detectChanges();
      await fixture.whenStable();
      fixture.detectChanges();

      expect(keys(cmp.filteredSpellsKnown())).toEqual([
        'spell:shield',
        'spell:magic-missile',
        'spell:fireball',
      ]);
      // Not selected there, so the prepared list still hides it.
      expect(keys(cmp.filteredSpellsPrepared())).not.toContain('spell:fireball');
      expect(el.textContent).toContain('Bola de Fogo (3º círculo, acima do nível)');
    });

    it('replaces the leveled lists with one line for a class that starts casting later', async () => {
      configure({ id: 'camp-1' });
      const { fixture, el } = await render();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cmp = fixture.componentInstance as any;

      cmp.fullForm.patchValue({ className: 'class:paladin', level: 1 });
      fixture.detectChanges();
      expect(el.textContent).toContain('O Paladino conjura magias a partir do nível 2.');
      expect(el.textContent).not.toContain('Magias preparadas');

      cmp.fullForm.patchValue({ level: 2 });
      fixture.detectChanges();
      expect(el.textContent).not.toContain('conjura magias a partir do nível');
      expect(el.textContent).toContain('Magias preparadas');
    });
  });

  it('sends chosen armor, weapons and cantrips as content keys, never typed text', async () => {
    configure({ id: 'camp-1' });
    const { fixture } = await render();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const cmp = fixture.componentInstance as any;

    cmp.fullForm.patchValue({
      name: 'Pensantus',
      race: 'race:gnome',
      className: 'class:wizard',
      background: 'background:acolyte',
      armor: 'equipment:leather-armor',
      weaponKeys: ['equipment:quarterstaff', 'equipment:dagger'],
    });
    cmp.toggleCantrip('spell:fire-bolt');
    cmp.toggleSpellKnown('spell:magic-missile');
    cmp.toggleSpellPrepared('spell:shield');

    await cmp.submit();

    const req = fake.createCharacterCalls[0];
    expect(req.full?.armor).toBe('equipment:leather-armor');
    expect(req.full?.weapons.sort()).toEqual(['equipment:dagger', 'equipment:quarterstaff']);
    expect(req.full?.cantrips).toEqual(['spell:fire-bolt']);
    expect(req.full?.spellsKnown).toEqual(['spell:magic-missile']);
    expect(req.full?.spellsPrepared).toEqual(['spell:shield']);
  });

  it('"Sem armadura" sends an empty armor key', async () => {
    configure({ id: 'camp-1' });
    const { fixture } = await render();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const cmp = fixture.componentInstance as any;

    cmp.fullForm.patchValue({
      name: 'Pensantus',
      race: 'race:gnome',
      className: 'class:wizard',
      background: 'background:acolyte',
      armor: '',
    });

    await cmp.submit();

    expect(fake.createCharacterCalls[0].full?.armor).toBe('');
  });

  it('never sends a stale spell pick that fell off the list after the class changed', async () => {
    configure({ id: 'camp-1' });
    const { fixture } = await render();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const cmp = fixture.componentInstance as any;

    cmp.fullForm.patchValue({
      name: 'Pensantus',
      race: 'race:gnome',
      className: 'class:wizard',
      background: 'background:acolyte',
    });
    cmp.selectedCantrips.set(new Set(['spell:fire-bolt', 'spell:not-on-any-list']));

    await cmp.submit();

    expect(fake.createCharacterCalls[0].full?.cantrips).toEqual(['spell:fire-bolt']);
  });

  it('shows the fiction notice on every free-text group of a full sheet', async () => {
    configure({ id: 'camp-1' });
    const { fixture, el } = await render();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const cmp = fixture.componentInstance as any;

    cmp.fullForm.patchValue({ className: 'class:wizard', background: 'custom' });
    fixture.detectChanges();

    // Custom background name, spell lists, and the equipment/languages/tools
    // group — three distinct free-text groups.
    expect(el.querySelectorAll('app-fiction-notice').length).toBe(3);
  });

  it('shows the fiction notice once on a basic (NPC) sheet, next to the description', async () => {
    configure({ id: 'camp-1', tipo: 'minion' });
    const { el } = await render();

    expect(el.querySelectorAll('app-fiction-notice').length).toBe(1);
  });

  it('sends the loaded revision when saving an edit', async () => {
    configure({ id: 'camp-1', characterId: 'char-1' });
    fake.loadCharacterForEditFn = () =>
      Promise.resolve({
        kind: 'player',
        revision: 7,
        full: {
          name: 'Pensantus',
          race: 'race:gnome',
          subrace: 'subrace:rock-gnome',
          className: 'class:wizard',
          subclassName: 'subclass:evocation',
          customSubclassName: '',
          level: 3,
          background: 'background:acolyte',
          customBackgroundName: '',
          customBackgroundSkills: null,
          skillProficiencies: ['skill:arcana'],
          expertiseSkillKeys: [],
          abilities: { str: 8, dex: 14, con: 16, int: 18, wis: 12, cha: 10 },
          extraAbilityBonuses: { str: 0, dex: 0, con: 0, int: 0, wis: 0, cha: 0 },
          hitPointsMethod: 'average',
          hitPointsRolls: [],
          isCaster: true,
          cantrips: ['Fire Bolt'],
          spellsKnown: [],
          spellsPrepared: [],
          armor: '',
          shield: false,
          weapons: [],
          equipmentText: '',
          languagesText: '',
          toolProficienciesText: '',
          experiencePoints: 0,
          alignment: '',
          customFeaturesText: '',
        },
        basic: null,
      });

    const { fixture } = await render();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const cmp = fixture.componentInstance as any;
    await cmp.submit();

    expect(fake.updateCharacterCalls.length).toBe(1);
    expect(fake.updateCharacterCalls[0].campaignId).toBe('camp-1');
    expect(fake.updateCharacterCalls[0].characterId).toBe('char-1');
    expect(fake.updateCharacterCalls[0].revision).toBe(7);
  });

  it('maps a locked sheet to its message when opening the editor for it', async () => {
    configure({ id: 'camp-1', characterId: 'char-1' });
    fake.loadCharacterForEditFn = () =>
      Promise.reject(
        new ConnectError('locked', Code.FailedPrecondition, undefined, [
          {
            desc: CharacterBlockedSchema,
            value: { reason: CharacterBlockedReason.SHEET_LOCKED, characterId: 'char-1' },
          },
        ]),
      );

    const { el } = await render();
    expect(el.textContent).toContain('travada');
  });

  it("wires a custom background's two chosen skills end to end", async () => {
    configure({ id: 'camp-1' });
    const { fixture } = await render();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const cmp = fixture.componentInstance as any;

    cmp.fullForm.patchValue({
      name: 'Pensantus',
      race: 'race:gnome',
      className: 'class:wizard',
      background: 'custom',
      customBackgroundName: 'Sábio',
    });
    cmp.toggleCustomBackgroundSkill('skill:arcana');
    cmp.toggleCustomBackgroundSkill('skill:history');

    await cmp.submit();

    const req = fake.createCharacterCalls[0];
    expect(req.full?.background).toBe('custom');
    expect(req.full?.customBackgroundName).toBe('Sábio');
    expect(req.full?.customBackgroundSkills).toEqual(['skill:arcana', 'skill:history']);
  });

  it("caps a custom background's skills at two", async () => {
    configure({ id: 'camp-1' });
    const { fixture } = await render();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const cmp = fixture.componentInstance as any;

    cmp.toggleCustomBackgroundSkill('skill:arcana');
    cmp.toggleCustomBackgroundSkill('skill:history');
    cmp.toggleCustomBackgroundSkill('skill:another'); // ignored: already at 2

    expect(cmp.customBackgroundSkills().size).toBe(2);
    expect(cmp.customBackgroundSkills().has('skill:another')).toBe(false);
  });

  it('only lets expertise apply to a proficient skill, and drops it if proficiency is removed', async () => {
    configure({ id: 'camp-1' });
    const { fixture } = await render();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const cmp = fixture.componentInstance as any;

    // Not proficient yet: toggling expertise does nothing.
    cmp.toggleExpertise('skill:arcana');
    expect(cmp.expertiseSkills().has('skill:arcana')).toBe(false);

    cmp.toggleSkill('skill:arcana');
    cmp.toggleExpertise('skill:arcana');
    expect(cmp.expertiseSkills().has('skill:arcana')).toBe(true);

    // Removing proficiency drops the now-invalid expertise too.
    cmp.toggleSkill('skill:arcana');
    expect(cmp.expertiseSkills().has('skill:arcana')).toBe(false);
  });

  it('sends expertise, manual ability bonuses, XP, alignment and custom features', async () => {
    configure({ id: 'camp-1' });
    const { fixture } = await render();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const cmp = fixture.componentInstance as any;

    cmp.fullForm.patchValue({
      name: 'Pensantus',
      race: 'race:gnome',
      className: 'class:wizard',
      background: 'background:acolyte',
      experiencePoints: 2700,
      alignment: 'neutral_good',
      customFeaturesText: 'Um truque de cartas que sempre erra.',
      extraAbilityBonuses: { str: 0, dex: 0, con: 1, int: 2, wis: 0, cha: 0 },
    });
    cmp.toggleSkill('skill:arcana');
    cmp.toggleExpertise('skill:arcana');

    await cmp.submit();

    const req = fake.createCharacterCalls[0];
    expect(req.full?.expertiseSkillKeys).toEqual(['skill:arcana']);
    expect(req.full?.extraAbilityBonuses).toEqual({
      str: 0,
      dex: 0,
      con: 1,
      int: 2,
      wis: 0,
      cha: 0,
    });
    expect(req.full?.experiencePoints).toBe(2700);
    expect(req.full?.alignment).toBe('neutral_good');
    expect(req.full?.customFeaturesText).toBe('Um truque de cartas que sempre erra.');
  });

  it('sends hit points rolls only for the "rolled" method, capped to the levels above the first', async () => {
    configure({ id: 'camp-1' });
    const { fixture } = await render();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const cmp = fixture.componentInstance as any;

    cmp.fullForm.patchValue({
      name: 'Pensantus',
      race: 'race:gnome',
      className: 'class:wizard',
      background: 'background:acolyte',
      level: 3,
      hitPointsMethod: 'rolled',
    });
    cmp.hitPointsRolls.set([4, 6, 99]); // one extra roll than level 3 needs (2)

    await cmp.submit();

    const req = fake.createCharacterCalls[0];
    expect(req.full?.hitPointsMethod).toBe('rolled');
    expect(req.full?.hitPointsRolls).toEqual([4, 6]);
  });

  it('sends the "average" hit points method when chosen, regardless of rolls typed earlier', async () => {
    configure({ id: 'camp-1' });
    const { fixture } = await render();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const cmp = fixture.componentInstance as any;

    cmp.fullForm.patchValue({
      name: 'Pensantus',
      race: 'race:gnome',
      className: 'class:wizard',
      background: 'background:acolyte',
      level: 3,
      hitPointsMethod: 'average',
    });
    cmp.hitPointsRolls.set([4, 6]);

    await cmp.submit();

    // The source (`CharacterEditorSourceLive.toFullSheetInit`) is what
    // actually blanks the rolls for "average" on the wire — see
    // character-editor-source.live.spec.ts.
    expect(fake.createCharacterCalls[0].full?.hitPointsMethod).toBe('average');
  });

  describe('the redesigned page', () => {
    function tabs(el: HTMLElement): HTMLElement[] {
      return Array.from(el.querySelectorAll<HTMLElement>('[role="tab"]'));
    }

    it('shows one tab per step, and Magias only for a caster class', async () => {
      configure({ id: 'camp-1' });
      const { fixture, el } = await render();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cmp = fixture.componentInstance as any;

      const names = () =>
        tabs(el).map((t) => t.querySelector('.stepper__label')?.textContent?.trim());
      expect(names()).toEqual(['Básico', 'Atributos', 'Perícias', 'Equipamento']);

      cmp.fullForm.patchValue({ className: 'class:wizard' });
      fixture.detectChanges();
      await fixture.whenStable();
      fixture.detectChanges();

      expect(names()).toEqual(['Básico', 'Atributos', 'Perícias', 'Magias', 'Equipamento']);
    });

    it('sends nothing on an invalid submit, lists what to fix and marks the step', async () => {
      configure({ id: 'camp-1' });
      const { fixture, el } = await render();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cmp = fixture.componentInstance as any;

      await cmp.submit();
      fixture.detectChanges();

      expect(fake.createCharacterCalls.length).toBe(0);
      const notice = el.querySelector('.mr-notice--danger[role="alert"]');
      expect(notice?.textContent).toContain('Corrija os campos marcados antes de criar.');
      expect(notice?.textContent).toContain(
        'Básico: Nome do personagem, Classe, Raça, Antecedente.',
      );
      expect(tabs(el)[0].textContent).toContain('(com erro)');
      expect(tabs(el)[1].textContent).not.toContain('(com erro)');

      // Fixing the fields clears the notice and the mark.
      cmp.fullForm.patchValue({
        name: 'Pensantus',
        race: 'race:gnome',
        className: 'class:wizard',
        background: 'background:acolyte',
      });
      fixture.detectChanges();
      expect(el.querySelector('.mr-notice--danger')).toBeNull();
      expect(tabs(el)[0].textContent).not.toContain('(com erro)');
    });

    it('opens the step of the first invalid field, and "Bônus manuais" for a bonus', async () => {
      configure({ id: 'camp-1' });
      const { fixture, el } = await render();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cmp = fixture.componentInstance as any;

      cmp.fullForm.patchValue({
        name: 'Pensantus',
        race: 'race:gnome',
        className: 'class:wizard',
        background: 'background:acolyte',
        extraAbilityBonuses: { str: 0, dex: 0, con: 11, int: 0, wis: 0, cha: 0 },
      });
      fixture.detectChanges();
      expect(cmp.bonusesOpen()).toBe(false);

      await cmp.submit();
      fixture.detectChanges();

      expect(fake.createCharacterCalls.length).toBe(0);
      expect(tabs(el)[1].getAttribute('aria-selected')).toBe('true');
      expect(cmp.bonusesOpen()).toBe(true);
      expect(el.querySelector('.mr-notice--danger')?.textContent).toContain(
        'Atributos: bônus manual de Constituição.',
      );
    });

    it("shows the server's reason in a danger notice when the save fails", async () => {
      configure({ id: 'camp-1' });
      fake.createCharacterFn = () =>
        Promise.reject(
          new ConnectError('exists', Code.FailedPrecondition, undefined, [
            {
              desc: CharacterBlockedSchema,
              value: { reason: CharacterBlockedReason.LIVING_CHARACTER_EXISTS, characterId: '' },
            },
          ]),
        );
      const { fixture, el } = await render();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cmp = fixture.componentInstance as any;

      cmp.fullForm.patchValue({
        name: 'Pensantus',
        race: 'race:gnome',
        className: 'class:wizard',
        background: 'background:acolyte',
      });
      await cmp.submit();
      fixture.detectChanges();

      const notice = el.querySelector('.mr-notice--danger[role="alert"]');
      expect(notice?.textContent).toContain('O personagem não foi criado.');
      expect(notice?.textContent).toContain('Você já tem um personagem vivo nesta campanha.');
    });

    it('says what the manual bonuses are for, and which are in use, while closed', async () => {
      configure({ id: 'camp-1' });
      const { fixture, el } = await render();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cmp = fixture.componentInstance as any;

      const summary = el.querySelector('.bonuses__summary');
      expect(summary?.textContent).toContain('Aumento de atributo, escolhas de raça, item mágico.');
      expect(el.querySelector('.bonuses__state')?.textContent).toContain('Nenhum em uso');

      cmp.fullForm.patchValue({
        extraAbilityBonuses: { str: 0, dex: 0, con: 1, int: 2, wis: 0, cha: 0 },
      });
      fixture.detectChanges();

      expect(el.querySelector('.bonuses__state')?.textContent).toContain(
        'Em uso: Constituição +1, Inteligência +2',
      );
    });

    it('never shares the exact field name of a score with a manual bonus', async () => {
      configure({ id: 'camp-1' });
      const { el } = await render();

      const labels = Array.from(el.querySelectorAll('app-ability-fields mat-label')).map((l) =>
        l.textContent?.replace(/\s+/g, ' ').trim(),
      );
      expect(labels.filter((l) => l === 'Força').length).toBe(1);
      expect(labels).toContain('Força (bônus manual)');
    });

    it('keeps "Nível" the only label with that word (sheet-lock.spec.ts matches it loosely)', async () => {
      configure({ id: 'camp-1' });
      const { el } = await render();

      const labels = Array.from(el.querySelectorAll('label, mat-label')).map(
        (l) => l.textContent?.toLowerCase() ?? '',
      );
      const withLevel = new Set(labels.filter((l) => l.includes('nível')).map((l) => l.trim()));
      expect(Array.from(withLevel)).toEqual(['nível']);
    });

    it('Cancelar goes back to the campaign when creating', async () => {
      configure({ id: 'camp-1' });
      const { el } = await render();

      const cancel = Array.from(el.querySelectorAll('a')).find(
        (a) => a.textContent?.trim() === 'Cancelar',
      );
      expect(cancel?.getAttribute('href')).toBe('/campanhas/camp-1');
    });

    it('Cancelar goes back to the sheet when editing', async () => {
      configure({ id: 'camp-1', characterId: 'char-9' });
      fake.loadCharacterForEditFn = () =>
        Promise.resolve({
          kind: 'minion',
          revision: 1,
          full: null,
          basic: {
            name: 'Goblin',
            hitPointsMax: 7,
            armorClass: 13,
            speedFt: 30,
            initiativeBonus: 2,
            attacks: [],
            legacyDamage: '',
            legacyAttackBonus: 0,
            description: '',
          },
        });
      const { el } = await render();

      const cancel = Array.from(el.querySelectorAll('a')).find(
        (a) => a.textContent?.trim() === 'Cancelar',
      );
      expect(cancel?.getAttribute('href')).toBe('/campanhas/camp-1/personagens/char-9');
      expect(el.querySelector('h1')?.textContent).toContain('Editar ficha');
      expect(el.querySelector('button[mat-flat-button]')?.textContent).toContain('Salvar ficha');
    });

    it('titles an NPC form "Criar NPC", and its primary action says the same', async () => {
      configure({ id: 'camp-1', tipo: 'minion' });
      const { el } = await render();

      expect(el.querySelector('h1')?.textContent).toContain('Criar NPC');
      expect(el.querySelector('.mr-page-lead')?.textContent).toContain('Minion: ficha curta');
      const primary = el.querySelector('button[mat-flat-button]');
      expect(primary?.textContent).toContain('Criar NPC');
    });

    it('lists what to fix on the short NPC form too', async () => {
      configure({ id: 'camp-1', tipo: 'minion' });
      const { fixture, el } = await render();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cmp = fixture.componentInstance as any;

      await cmp.submit();
      fixture.detectChanges();

      expect(fake.createCharacterCalls.length).toBe(0);
      expect(el.querySelector('.mr-notice--danger')?.textContent).toContain('Nome do personagem.');
    });

    it('adds and removes attack cards on the short NPC form, at most three', async () => {
      configure({ id: 'camp-1', tipo: 'minion' });
      const { fixture, el } = await render();
      const add = () =>
        Array.from(el.querySelectorAll('button')).find((b) =>
          b.textContent?.includes('Adicionar ataque'),
        ) as HTMLButtonElement;
      expect(el.querySelectorAll('app-npc-attack-card').length).toBe(0);

      for (let i = 0; i < 3; i++) {
        add().click();
        fixture.detectChanges();
        await fixture.whenStable();
      }
      expect(el.querySelectorAll('app-npc-attack-card').length).toBe(3);
      expect(add().disabled).toBe(true);
      expect(el.textContent).toContain('Máximo de 3 ataques');
      expect(el.textContent).toContain('3 de 3 ataques');

      (el.querySelector('[aria-label="Remover o ataque 2"]') as HTMLButtonElement).click();
      fixture.detectChanges();
      await fixture.whenStable();
      expect(el.querySelectorAll('app-npc-attack-card').length).toBe(2);
      expect(add().disabled).toBe(false);
      expect(el.querySelector('[role="status"]')?.textContent).toContain('Ataque 2 removido.');
    });

    it('lists the invalid attack fields by card, and does not save', async () => {
      configure({ id: 'camp-1', tipo: 'minion' });
      const { fixture, el } = await render();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cmp = fixture.componentInstance as any;
      cmp.basicForm.controls.name.setValue('Goblin');
      cmp.basicForm.controls.attacks.push(createAttackGroup(TestBed.inject(FormBuilder)));

      await cmp.submit();
      fixture.detectChanges();

      expect(fake.createCharacterCalls.length).toBe(0);
      expect(el.querySelector('.mr-notice--danger')?.textContent).toContain(
        'Ataque 1: Nome do ataque, Ataque 1: Tipo de dano.',
      );
    });

    it('gives the loading spinner an accessible name', async () => {
      configure({ id: 'camp-1' });
      fake.loadCatalogFn = () => new Promise(() => undefined);
      const fixture = TestBed.createComponent(CharacterEditor);
      fixture.detectChanges();
      const el = fixture.nativeElement as HTMLElement;

      expect(el.querySelector('h1')?.textContent).toContain('Criar personagem');
      expect(el.querySelector('mat-spinner')?.getAttribute('aria-label')).toBe(
        'Carregando o formulário',
      );
    });
  });

  describe('rolls and the spell "?" (MR-004)', () => {
    afterEach(() => {
      document.querySelectorAll('.cdk-overlay-container *').forEach((n) => n.remove());
    });

    it('refuses to save while a rolled result has no ability, and says so', async () => {
      configure({ id: 'camp-1' });
      const { fixture, el } = await render();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cmp = fixture.componentInstance as any;
      cmp.fullForm.patchValue({
        name: 'Pensantus',
        race: 'race:gnome',
        className: 'class:wizard',
        background: 'background:acolyte',
      });
      cmp.abilitiesIncomplete.set(true);

      await cmp.submit();
      fixture.detectChanges();

      expect(fake.createCharacterCalls.length).toBe(0);
      expect(el.querySelector('.mr-notice--danger')?.textContent).toContain(
        'Atributos: coloque cada resultado num atributo.',
      );

      cmp.abilitiesIncomplete.set(false);
      await cmp.submit();
      expect(fake.createCharacterCalls.length).toBe(1);
    });

    it('feeds the hit-point preview the final Constitution: base, race, subrace and manual bonus', async () => {
      configure({ id: 'camp-1' });
      const { fixture, el } = await render();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cmp = fixture.componentInstance as any;
      cmp.fullForm.patchValue({
        race: 'race:gnome',
        subrace: 'subrace:rock-gnome', // +1 CON in the fixture
        className: 'class:wizard', // d6
        level: 3,
        hitPointsMethod: 'rolled',
        abilities: { con: 14 },
        extraAbilityBonuses: { con: 1 },
      });
      fixture.detectChanges();
      expect(cmp.finalConstitution()).toBe(16);
      expect(cmp.hitDie()).toBe(6);
      const labels = Array.from(el.querySelectorAll('app-hit-points-rolls mat-label')).map((l) =>
        l.textContent?.trim(),
      );
      expect(labels).toEqual(['Nível 2 (1d6)', 'Nível 3 (1d6)']);
      expect(el.querySelector('app-hit-points-rolls .hp__note')?.textContent).toContain(
        'Constituição 16 (+3 por nível)',
      );
    });

    it('asks for a class before offering hit-point dice', async () => {
      configure({ id: 'camp-1' });
      const { fixture, el } = await render();
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cmp = fixture.componentInstance as any;
      cmp.fullForm.patchValue({ level: 3, hitPointsMethod: 'rolled' });
      fixture.detectChanges();
      expect(el.querySelector('app-hit-points-rolls')).toBeNull();
      expect(el.textContent).toContain('Escolha a classe no passo Básico');
    });

    async function openKnock(fixture: ComponentFixture<CharacterEditor>, el: HTMLElement) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const cmp = fixture.componentInstance as any;
      cmp.fullForm.patchValue({ className: 'class:wizard', level: 3 });
      fixture.detectChanges();
      const help = el.querySelector<HTMLButtonElement>(
        'button[aria-label="Descrição de Mísseis Mágicos"]',
      )!;
      help.focus();
      help.click();
      await flush();
      await fixture.whenStable();
      fixture.detectChanges();
      return help;
    }

    it('opens the spell description in a dialog, fetched once per spell, and gives focus back on close', async () => {
      configure({ id: 'camp-1' });
      const { fixture, el } = await render();
      const help = await openKnock(fixture, el);

      const dialog = document.querySelector('app-spell-details')!;
      expect(dialog.textContent).toContain('Alcance');
      expect(dialog.textContent).toContain('18 m');
      expect(dialog.querySelector('[lang=en]')).not.toBeNull();
      expect(fake.loadSpellDetailsCalls).toEqual(['spell:magic-missile']);

      Array.from(dialog.querySelectorAll('button'))
        .find((b) => b.textContent?.trim() === 'Fechar')!
        .click();
      // The dialog leaves with a short animation: wait for it.
      for (let i = 0; i < 40 && document.querySelector('app-spell-details'); i++) {
        await new Promise((resolve) => setTimeout(resolve, 25));
      }
      fixture.detectChanges();
      expect(document.querySelector('app-spell-details')).toBeNull();
      expect(document.activeElement).toBe(help);

      help.click();
      await flush();
      fixture.detectChanges();
      expect(fake.loadSpellDetailsCalls.length).toBe(1);
    });

    it('asks again after a failed fetch', async () => {
      configure({ id: 'camp-1' });
      fake.loadSpellDetailsFn = () => Promise.reject(new Error('offline'));
      const { fixture, el } = await render();
      await openKnock(fixture, el);
      expect(document.querySelector('app-spell-details [role=alert]')).not.toBeNull();
      expect(fake.loadSpellDetailsCalls.length).toBe(1);
    });
  });
});
