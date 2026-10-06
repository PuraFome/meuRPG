import { Injectable } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { of } from 'rxjs';

import { CharacterEditor } from './character-editor';
import {
  CharacterEditorSource,
  CharacterForEdit,
  ClassOptionVm,
  CreateCharacterInput,
  RulesCatalogVm,
  SpellOptionVm,
  SubclassOptionVm,
  UpdateCharacterInput,
} from './character-editor.types';
import { TOOLS_AND_LANGUAGES } from './tools-and-languages';

// The editor with the table's own content (MR-025, RN-23, slice 10.12b): a table class, race and
// background, the "Outro" background, a sheet of several classes (E10-02 states 5 and 6) and the
// spell step per class (E10-11 state 4). The numbers are the server's: these specs give the
// editor a catalog, as the server would, and read the request it builds.

const SRD = { fromTable: false, archived: false };
const TABLE = { fromTable: true, archived: false };

const cls = (over: Partial<ClassOptionVm> & Pick<ClassOptionVm, 'key' | 'namePt'>): ClassOptionVm => ({
  hitDie: 8,
  isCaster: false,
  preparation: null,
  subclasses: [],
  subclassLevel: 3,
  spellcastingFirstLevel: 0,
  maxSpellLevelByLevel: [],
  skillChoose: 2,
  savingThrows: ['str', 'wis'],
  spellListClassKey: '',
  ...SRD,
  ...over,
});
const sub = (over: Partial<SubclassOptionVm> & Pick<SubclassOptionVm, 'key' | 'namePt'>): SubclassOptionVm => ({ casting: null, ...SRD, ...over });
const spell = (key: string, namePt: string, level: number, classKeys: string[]): SpellOptionVm => ({ key, namePt, level, classKeys, fromTable: key.endsWith('@mesa') });

const CIRCLES = [1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 9, 9];

function catalog(): RulesCatalogVm {
  return {
    races: [
      { key: 'race:gnome', namePt: 'Gnomo', constitutionBonus: 0, choiceBonuses: [], subraces: [], ...SRD },
      { key: 'race:corujeiro@mesa', namePt: 'Corujeiro', constitutionBonus: 0, choiceBonuses: [2, 1], subraces: [], ...TABLE },
    ],
    classes: [
      cls({ key: 'class:wizard', namePt: 'Mago', hitDie: 6, isCaster: true, preparation: 'spellbook', spellcastingFirstLevel: 1, maxSpellLevelByLevel: CIRCLES, spellListClassKey: 'class:wizard', subclassLevel: 2, savingThrows: ['int', 'wis'], subclasses: [sub({ key: 'subclass:evocation', namePt: 'Escola de Evocação' }), sub({ key: 'subclass:ink@mesa', namePt: 'Tradição da Tinta', ...TABLE })] }),
      cls({ key: 'class:cleric', namePt: 'Clérigo', isCaster: true, preparation: 'prepared', spellcastingFirstLevel: 1, maxSpellLevelByLevel: CIRCLES, spellListClassKey: 'class:cleric', subclassLevel: 1, subclasses: [sub({ key: 'subclass:life', namePt: 'Domínio da Vida' }), sub({ key: 'subclass:path@mesa', namePt: 'Domínio do Caminho', ...TABLE })] }),
      cls({ key: 'class:guardiao@mesa', namePt: 'Guardião do Vale', hitDie: 10, skillChoose: 2, savingThrows: ['str', 'wis'], ...TABLE }),
      cls({
        key: 'class:fighter',
        namePt: 'Guerreiro',
        hitDie: 10,
        subclasses: [
          sub({ key: 'subclass:champion', namePt: 'Campeão' }),
          sub({ key: 'subclass:ink-blade@mesa', namePt: 'Lâmina de Tinta', ...TABLE, casting: { preparation: 'known', listClassKey: 'class:wizard', firstLevel: 3, maxSpellLevelByLevel: [0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 3, 3, 4, 4, 4, 4] } }),
        ],
      }),
      cls({ key: 'class:bard', namePt: 'Bardo' }),
      cls({ key: 'class:druid', namePt: 'Druida' }),
    ],
    backgrounds: [
      { key: 'background:acolyte', namePt: 'Acólito', equipmentPt: '', ...SRD },
      { key: 'background:cartografo@mesa', namePt: 'Cartógrafo do Vale', equipmentPt: 'Uma luneta e um rolo de corda', ...TABLE },
    ],
    skills: [
      { key: 'skill:arcana', namePt: 'Arcanismo', ability: 'int' },
      { key: 'skill:survival', namePt: 'Sobrevivência', ability: 'wis' },
      { key: 'skill:perception', namePt: 'Percepção', ability: 'wis' },
    ],
    armor: [],
    weapons: [],
    spells: [
      spell('spell:fire-bolt', 'Raio de Fogo', 0, ['class:wizard']),
      spell('spell:shield', 'Escudo Arcano', 1, ['class:wizard']),
      spell('spell:detect-magic', 'Detectar Magia', 1, ['class:wizard', 'class:cleric']),
      spell('spell:bless', 'Bênção', 1, ['class:cleric']),
      spell('spell:ink-blade@mesa', 'Lâmina de Nanquim', 1, ['class:wizard']),
      spell('spell:animal-friendship', 'Amizade Animal', 1, ['class:bard', 'class:druid']),
    ],
    toolsAndLanguages: TOOLS_AND_LANGUAGES,
    challengeRatings: [],
  };
}

@Injectable()
class FakeSource {
  created: CreateCharacterInput[] = [];
  updated: UpdateCharacterInput[] = [];
  forEdit: CharacterForEdit | null = null;
  loadCatalog() {
    return Promise.resolve(catalog());
  }
  loadSpellDetails() {
    return Promise.reject(new Error('unused'));
  }
  loadCharacterForEdit() {
    return this.forEdit ? Promise.resolve(this.forEdit) : Promise.reject(new Error('not stubbed'));
  }
  loadAbilityTable() {
    return Promise.resolve(null);
  }
  rollAbilityScores() {
    return Promise.reject(new Error('unused'));
  }
  createCharacter(input: CreateCharacterInput) {
    this.created.push(input);
    return Promise.resolve({ characterId: 'new' });
  }
  updateCharacter(input: UpdateCharacterInput) {
    this.updated.push(input);
    return Promise.resolve({ revision: 2 });
  }
}

const flush = () => new Promise<void>((resolve) => setTimeout(resolve, 0));

async function render(params: Record<string, string> = { id: 'camp-1' }) {
  TestBed.configureTestingModule({
    imports: [CharacterEditor],
    providers: [
      provideRouter([]),
      { provide: CharacterEditorSource, useClass: FakeSource },
      { provide: ActivatedRoute, useValue: { paramMap: of(convertToParamMap(params)) } },
    ],
  });
  const fake = TestBed.inject(CharacterEditorSource) as unknown as FakeSource;
  const fixture = TestBed.createComponent(CharacterEditor);
  fixture.detectChanges();
  await flush();
  await fixture.whenStable();
  fixture.detectChanges();
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  return { fixture, fake, el: fixture.nativeElement as HTMLElement, cmp: fixture.componentInstance as any };
}

async function settle(fixture: ComponentFixture<CharacterEditor>) {
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
}

async function openStep(fixture: ComponentFixture<CharacterEditor>, label: string) {
  const tab = Array.from((fixture.nativeElement as HTMLElement).querySelectorAll<HTMLElement>('[role="tab"]')).find((t) => t.textContent?.includes(label));
  tab!.click();
  await settle(fixture);
}

const button = (el: HTMLElement, text: string) => Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) => b.textContent?.includes(text));

describe('a table class, race and background in the editor (E10-02 state 5)', () => {
  it('sends the keys of the table content, never their names', async () => {
    const { fixture, fake, cmp } = await render();
    cmp.fullForm.patchValue({ name: 'Ícaro', race: 'race:corujeiro@mesa', className: 'class:guardiao@mesa', level: 1, background: 'background:cartografo@mesa' });
    await cmp.submit();
    const full = fake.created[0].full!;
    expect([full.race, full.className, full.background]).toEqual(['race:corujeiro@mesa', 'class:guardiao@mesa', 'background:cartografo@mesa']);
    expect(full.extraClasses).toEqual([]);
  });

  it("tells the class's skill count and saving throws from the server's entry, and where a race's free bonuses go", async () => {
    const { fixture, el, cmp } = await render();
    cmp.fullForm.patchValue({ race: 'race:corujeiro@mesa', className: 'class:guardiao@mesa' });
    await settle(fixture);
    const text = (el.textContent ?? '').replace(/\s+/g, ' ');
    expect(text).toContain('Dá +2 e +1 nos atributos que você escolher');
    await openStep(fixture, 'Perícias');
    expect((el.textContent ?? '').replace(/\s+/g, ' ')).toContain('O Guardião do Vale escolhe 2 perícias ao começar e dá proficiência nos testes de resistência de Força e Sabedoria.');
  });

  it("shows a table background's equipment as the master wrote it", async () => {
    const { fixture, el, cmp } = await render();
    cmp.fullForm.patchValue({ background: 'background:cartografo@mesa' });
    await settle(fixture);
    expect((el.textContent ?? '')).toContain('Equipamento: Uma luneta e um rolo de corda');
  });

  it('marks the entries of the table with "Da mesa" in the lists, and nothing else', async () => {
    const { fixture, el } = await render();
    const trigger = (label: string) => Array.from(el.querySelectorAll<HTMLElement>('mat-form-field')).find((f) => f.querySelector('mat-label')?.textContent?.includes(label))!.querySelector<HTMLElement>('.mat-mdc-select-trigger')!;
    trigger('Classe').click();
    await settle(fixture);
    const options = Array.from(document.querySelectorAll('mat-option')).map((o) => (o.textContent ?? '').replace(/menu_book|inventory_2/g, '').replace(/\s+/g, ' ').trim());
    expect(options).toContain('Guardião do Vale Da mesa');
    expect(options).toContain('Mago');
    expect(options.filter((o) => o.includes('Da mesa'))).toEqual(['Guardião do Vale Da mesa']);
  });
});

describe('the "Outro" background (E10-02 state 5, question 82)', () => {
  it('asks for the name, two skills, two tools or languages, the feature and the equipment, and sends them all', async () => {
    const { fixture, fake, el, cmp } = await render();
    cmp.fullForm.patchValue({
      name: 'Davi',
      race: 'race:gnome',
      className: 'class:fighter',
      background: 'custom',
      customBackgroundName: 'Batedor de torre',
      customBackgroundFeatureName: 'Olho no horizonte',
      customBackgroundFeatureText: 'Você sempre acha o ponto mais alto de um lugar.',
      customBackgroundEquipment: 'Uma luneta, um rolo de corda e 10 PO',
    });
    cmp.toggleCustomBackgroundSkill('skill:perception');
    cmp.toggleCustomBackgroundSkill('skill:survival');
    cmp.setCustomProficiencies(['proficiency:cartographers-tools', 'language:elvish', 'language:dwarvish']);
    await settle(fixture);
    const text = (el.textContent ?? '').replace(/\s+/g, ' ');
    expect(text).toContain('Personalizar um antecedente');
    expect(text).toContain('2 de 2 escolhidas');
    await cmp.submit();
    expect(fake.created[0].full).toMatchObject({
      background: 'custom',
      customBackgroundName: 'Batedor de torre',
      customBackgroundSkills: ['skill:perception', 'skill:survival'],
      // A third is never taken: two, like the skills.
      customBackgroundProficiencies: ['proficiency:cartographers-tools', 'language:elvish'],
      customBackgroundFeatureName: 'Olho no horizonte',
      customBackgroundFeatureText: 'Você sempre acha o ponto mais alto de um lugar.',
      customBackgroundEquipment: 'Uma luneta, um rolo de corda e 10 PO',
    });
  });

  it('sends fewer than two as they are: the server shows an issue on the sheet, never an error', async () => {
    const { fake, cmp } = await render();
    cmp.fullForm.patchValue({ name: 'Davi', race: 'race:gnome', className: 'class:fighter', background: 'custom', customBackgroundName: 'Batedor' });
    cmp.setCustomProficiencies(['language:elvish']);
    await cmp.submit();
    expect(fake.created[0].full).toMatchObject({ customBackgroundProficiencies: ['language:elvish'], customBackgroundSkills: null });
  });

  it('offers the tools and the languages in two groups, in Portuguese', () => {
    const tools = TOOLS_AND_LANGUAGES.filter((t) => t.kind === 'tool').map((t) => t.namePt);
    const languages = TOOLS_AND_LANGUAGES.filter((t) => t.kind === 'language').map((t) => t.namePt);
    expect(tools).toContain('Ferramentas de cartógrafo');
    expect(languages).toContain('Élfico');
    expect(TOOLS_AND_LANGUAGES.every((t) => t.key.startsWith(t.kind === 'tool' ? 'proficiency:' : 'language:'))).toBe(true);
  });
});

describe('several classes at creation (E10-02 state 6)', () => {
  async function maga() {
    const r = await render();
    r.cmp.fullForm.patchValue({ name: 'Corvina', race: 'race:gnome', className: 'class:wizard', level: 3, subclassName: 'subclass:ink@mesa', background: 'background:acolyte' });
    await settle(r.fixture);
    return r;
  }

  it('adds a block per class, with the total level and the table\'s subclass in each', async () => {
    const { fixture, fake, el, cmp } = await maga();
    expect(el.querySelectorAll('app-class-block').length).toBe(0);
    button(el, 'Adicionar classe')!.click();
    await settle(fixture);
    const blocks = el.querySelectorAll('app-class-block');
    expect(blocks.length).toBe(2);
    expect(blocks[0].textContent).toContain('Classe 1');
    expect(blocks[1].textContent).toContain('Classe 2');
    // The total is read, never typed: the first class's level and the second's.
    cmp.changeBlock(1, { classKey: 'class:cleric', level: 1, subclassKey: 'subclass:path@mesa' });
    await settle(fixture);
    expect(cmp.totalLevelValue()).toBe(4);
    expect(el.querySelector('.xp-read__value')?.textContent?.trim()).toBe('4');
    await cmp.submit();
    expect(fake.created[0].full).toMatchObject({
      className: 'class:wizard',
      level: 3,
      subclassName: 'subclass:ink@mesa',
      extraClasses: [{ classKey: 'class:cleric', level: 1, subclassKey: 'subclass:path@mesa', customSubclassName: '' }],
    });
  });

  it('keeps the subclass shut until the class level that chooses it', async () => {
    const { fixture, el, cmp } = await render();
    cmp.fullForm.patchValue({ className: 'class:wizard', level: 1 });
    cmp.addClass();
    cmp.changeBlock(1, { classKey: 'class:fighter', level: 2 });
    await settle(fixture);
    const second = el.querySelectorAll('app-class-block')[1];
    expect(second.textContent).toContain('O Guerreiro escolhe a subclasse no nível 3.');
    expect(second.querySelector('mat-select[aria-disabled="true"], .mat-mdc-select-disabled')).not.toBeNull();
    cmp.changeBlock(1, { level: 3 });
    await settle(fixture);
    expect(el.querySelectorAll('app-class-block')[1].textContent).not.toContain('O Guerreiro escolhe a subclasse no nível 3.');
  });

  it('removes a class and refuses a blank block or a total above 20 before saving', async () => {
    const { fixture, fake, el, cmp } = await maga();
    cmp.addClass();
    await settle(fixture);
    await cmp.submit();
    expect(fake.created).toHaveLength(0);
    expect(cmp.invalidSummary()).toContain('Classe 2');
    cmp.changeBlock(1, { classKey: 'class:cleric', level: 19 });
    await settle(fixture);
    expect(cmp.totalLevelProblem()).toBe('O nível total vai até 20.');
    await cmp.submit();
    expect(fake.created).toHaveLength(0);
    expect(cmp.invalidSummary()).toContain('o nível total vai até 20');
    // "Remover" takes the block out, and the sheet is a one-class sheet again.
    button(el, 'Remover')!.click();
    await settle(fixture);
    expect(el.querySelectorAll('app-class-block').length).toBe(0);
    await cmp.submit();
    expect(fake.created).toHaveLength(1);
    expect(fake.created[0].full?.extraClasses).toEqual([]);
  });

  it('rolls the hit points level by level with the die of the class that gives it', async () => {
    const { fixture, cmp } = await maga();
    cmp.changeBlock(0, { level: 2 });
    cmp.addClass();
    cmp.changeBlock(1, { classKey: 'class:cleric', level: 1 });
    cmp.fullForm.patchValue({ hitPointsMethod: 'rolled' });
    await settle(fixture);
    // Mago 2 (d6), Clérigo 1 (d8): the 2nd level of the wizard, then the cleric's first.
    expect(cmp.levelDice()).toEqual([6, 8]);
    expect(cmp.rollsNeeded()).toBe(2);
  });
});

describe('the spell step per class (E10-11 state 4)', () => {
  async function corvina() {
    const r = await render();
    r.cmp.fullForm.patchValue({ name: 'Corvina', race: 'race:gnome', className: 'class:wizard', level: 3, background: 'background:acolyte' });
    r.cmp.addClass();
    r.cmp.changeBlock(1, { classKey: 'class:cleric', level: 1 });
    await settle(r.fixture);
    await openStep(r.fixture, 'Magias');
    return r;
  }

  it("gives each class its own section: the wizard's list up to the 2nd circle, the cleric's up to the 1st", async () => {
    const { el } = await corvina();
    const headings = Array.from(el.querySelectorAll('.spell-section__title')).map((h) => h.textContent?.trim());
    expect(headings).toEqual(['Mago · até o 2º círculo', 'Clérigo · até o 1º círculo']);
    const sections = Array.from(el.querySelectorAll('.spell-section'));
    const names = (s: Element) => Array.from(s.querySelectorAll('mat-checkbox')).map((c) => c.textContent?.replace(/\s+/g, ' ').trim());
    expect(names(sections[0])).toEqual(['Raio de Fogo', 'Detectar Magia (1º círculo)', 'Escudo Arcano (1º círculo)', 'Lâmina de Nanquim (1º círculo)', 'Detectar Magia (1º círculo)', 'Escudo Arcano (1º círculo)', 'Lâmina de Nanquim (1º círculo)']);
    expect(names(sections[1])).toEqual(['Bênção (1º círculo)', 'Detectar Magia (1º círculo)']);
  });

  it('greys out a spell that no class of the sheet lists, with the reason and a link to "Magias"', async () => {
    const { fixture, el, cmp } = await corvina();
    cmp.setSpellFilter(1, 'prepared', 'amizade');
    await settle(fixture);
    const clericSection = el.querySelectorAll('.spell-section')[1];
    const out = clericSection.querySelector('.picker__out')!;
    expect(out.textContent).toContain('Amizade Animal');
    expect(out.textContent).toContain('Fora da lista das suas classes');
    expect(out.textContent).toContain('Esta magia é de Bardo e Druida. Você a lê em “Magias”, mas não a escolhe nesta ficha.');
    expect(out.querySelector('mat-checkbox')).toBeNull();
    expect(out.querySelector('a')?.getAttribute('href')).toBe('/campanhas/camp-1/magias');
  });

  it("sends only picks that are on a list of the sheet, from the table's spells too", async () => {
    const { fake, cmp } = await corvina();
    cmp.toggleSpellKnown('spell:ink-blade@mesa');
    cmp.toggleSpellPrepared('spell:bless');
    cmp.toggleSpellPrepared('spell:animal-friendship');
    await cmp.submit();
    expect(fake.created[0].full?.spellsKnown).toEqual(['spell:ink-blade@mesa']);
    expect(fake.created[0].full?.spellsPrepared).toEqual(['spell:bless']);
  });

  it("offers a third caster's subclass the list it casts from, from its level on", async () => {
    const { fixture, el, cmp } = await render();
    cmp.fullForm.patchValue({ name: 'Rúnico', race: 'race:gnome', className: 'class:fighter', level: 2, subclassName: 'subclass:ink-blade@mesa', background: 'background:acolyte' });
    await settle(fixture);
    // Level 2: the subclass does not cast yet, so there is no Magias step.
    expect(Array.from(el.querySelectorAll('[role="tab"]')).some((t) => t.textContent?.includes('Magias'))).toBe(false);
    cmp.fullForm.patchValue({ level: 3 });
    await settle(fixture);
    await openStep(fixture, 'Magias');
    const names = Array.from(el.querySelectorAll('.spell-section mat-checkbox')).map((c) => c.textContent?.replace(/\s+/g, ' ').trim());
    expect(names).toContain('Escudo Arcano (1º círculo)');
    expect(names).toContain('Lâmina de Nanquim (1º círculo)');
    expect(names).not.toContain('Bênção (1º círculo)');
  });
});

describe('an edit of a sheet of several classes', () => {
  it('opens every class in its block and saves them all in the order they were', async () => {
    TestBed.configureTestingModule({
      imports: [CharacterEditor],
      providers: [
        provideRouter([]),
        { provide: CharacterEditorSource, useClass: FakeSource },
        { provide: ActivatedRoute, useValue: { paramMap: of(convertToParamMap({ id: 'camp-1', characterId: 'ch-1' })) } },
      ],
    });
    const fake = TestBed.inject(CharacterEditorSource) as unknown as FakeSource;
    fake.forEdit = {
      kind: 'player',
      revision: 3,
      blocked: null,
      sheetLocked: false,
      basic: null,
      grantedSpellKeys: ['spell:bless'],
      full: {
        name: 'Corvina',
        race: 'race:gnome',
        subrace: '',
        className: 'class:wizard',
        subclassName: 'subclass:ink@mesa',
        customSubclassName: '',
        level: 3,
        background: 'background:acolyte',
        customBackgroundName: '',
        customBackgroundSkills: null,
        customBackgroundProficiencies: [],
        customBackgroundFeatureName: '',
        customBackgroundFeatureText: '',
        customBackgroundEquipment: '',
        extraClasses: [{ classKey: 'class:cleric', level: 1, subclassKey: 'subclass:path@mesa', customSubclassName: '' }],
        skillProficiencies: [],
        expertiseSkillKeys: [],
        abilities: { str: 10, dex: 10, con: 10, int: 10, wis: 10, cha: 10 },
        extraAbilityBonuses: { str: 0, dex: 0, con: 0, int: 0, wis: 0, cha: 0 },
        hitPointsMethod: 'average',
        hitPointsRolls: [],
        isCaster: true,
        cantrips: [],
        spellsKnown: [],
        spellsPrepared: [],
        armor: '',
        shield: false,
        weapons: [],
        equipmentText: '',
        languagesText: '',
        toolProficienciesText: '',
        experiencePoints: 2700,
        challengeRating: '',
        xpValue: 0,
        portraitImageId: '',
        alignment: '',
        customFeaturesText: '',
      },
    };
    const fixture = TestBed.createComponent(CharacterEditor);
    fixture.detectChanges();
    await flush();
    await settle(fixture);
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelectorAll('app-class-block').length).toBe(2);
    expect(el.querySelector('.xp-read__value')?.textContent?.trim()).toBe('4');

    // The spells the subclass always prepares are read, with the word and the lock, never a checkbox.
    await openStep(fixture, 'Magias');
    const granted = el.querySelector('.granted')!;
    expect(granted.textContent).toContain('Bênção');
    expect(granted.textContent).toContain('Sempre preparada');
    expect(granted.querySelector('mat-checkbox')).toBeNull();

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    await (fixture.componentInstance as any).submit();
    expect(fake.updated[0].full?.extraClasses).toEqual([{ classKey: 'class:cleric', level: 1, subclassKey: 'subclass:path@mesa', customSubclassName: '' }]);
  });
});
