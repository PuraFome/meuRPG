import { Injectable } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, Router, convertToParamMap } from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';
import { of } from 'rxjs';

import { ABILITY_KEYS } from '../../core/characters/characters.types';
import { CharacterSheetPage } from './character-sheet';
import {
  BasicSheetVm,
  CharacterSheetSource,
  CharacterSheetVm,
  CharacterStoryVm,
  FullSheetVm,
} from './character-sheet.types';

@Injectable()
class FakeCharacterSheetSource {
  getCharacterSheetFn: (campaignId: string, characterId: string) => Promise<CharacterSheetVm> = () =>
    Promise.reject(new Error('not stubbed'));
  getMasterNotesCalls: string[] = [];
  getMasterNotesFn: (campaignId: string, characterId: string) => Promise<string> = () =>
    Promise.resolve('');
  updateMasterNotesFn: (campaignId: string, characterId: string, notes: string) => Promise<void> =
    () => Promise.resolve();
  markCharacterDeadFn: (campaignId: string, characterId: string) => Promise<CharacterSheetVm> = () =>
    Promise.reject(new Error('not stubbed'));
  updateCharacterStoryFn: (
    campaignId: string,
    characterId: string,
    revision: number,
    story: CharacterStoryVm,
  ) => Promise<CharacterSheetVm> = () => Promise.reject(new Error('not stubbed'));
  setStoryEditingAllowedCalls: Array<{ id: string; allowed: boolean }> = [];
  setStoryEditingAllowedFn: (
    campaignId: string,
    characterId: string,
    allowed: boolean,
  ) => Promise<CharacterSheetVm> = () => Promise.reject(new Error('not stubbed'));

  approveCharacterFn: (campaignId: string, characterId: string) => Promise<CharacterSheetVm> = () =>
    Promise.reject(new Error('not stubbed'));
  rejectCharacterFn: (campaignId: string, characterId: string) => Promise<void> = () =>
    Promise.reject(new Error('not stubbed'));
  rejectCharacterCalls: string[] = [];

  getCharacterSheet(campaignId: string, characterId: string): Promise<CharacterSheetVm> {
    return this.getCharacterSheetFn(campaignId, characterId);
  }
  approveCharacter(campaignId: string, characterId: string): Promise<CharacterSheetVm> {
    return this.approveCharacterFn(campaignId, characterId);
  }
  rejectCharacter(campaignId: string, characterId: string): Promise<void> {
    this.rejectCharacterCalls.push(characterId);
    return this.rejectCharacterFn(campaignId, characterId);
  }
  getMasterNotes(campaignId: string, characterId: string): Promise<string> {
    this.getMasterNotesCalls.push(characterId);
    return this.getMasterNotesFn(campaignId, characterId);
  }
  updateMasterNotes(campaignId: string, characterId: string, notes: string): Promise<void> {
    return this.updateMasterNotesFn(campaignId, characterId, notes);
  }
  markCharacterDead(campaignId: string, characterId: string): Promise<CharacterSheetVm> {
    return this.markCharacterDeadFn(campaignId, characterId);
  }
  updateCharacterStory(
    campaignId: string,
    characterId: string,
    revision: number,
    story: CharacterStoryVm,
  ): Promise<CharacterSheetVm> {
    return this.updateCharacterStoryFn(campaignId, characterId, revision, story);
  }
  setStoryEditingAllowed(
    campaignId: string,
    characterId: string,
    allowed: boolean,
  ): Promise<CharacterSheetVm> {
    this.setStoryEditingAllowedCalls.push({ id: characterId, allowed });
    return this.setStoryEditingAllowedFn(campaignId, characterId, allowed);
  }
}

function emptyStory(): CharacterStoryVm {
  return {
    personality: { traits: '', ideals: '', bonds: '', flaws: '' },
    appearance: { age: '', height: '', weight: '', eyes: '', skin: '', hair: '', description: '' },
    backstory: '',
    allies: '',
  };
}

function fullSheet(overrides: Partial<FullSheetVm> = {}): FullSheetVm {
  return {
    kind: 'full',
    abilities: ABILITY_KEYS.map((key) => ({ key, score: 10, modifier: 0 })),
    proficiencyBonus: 2,
    savingThrows: [],
    skills: [],
    passivePerception: 10,
    passiveInvestigation: 10,
    passiveInsight: 10,
    initiative: 0,
    armorClass: 10,
    armorClassDescription: 'Sem armadura',
    wearsArmor: false,
    hasShield: false,
    hitPointsMax: 10,
    hitDice: '1d6',
    speedWalkFt: 25,
    senses: [],
    attacks: [],
    spellcasting: [],
    spellSlots: [],
    cantripNames: [],
    spellNames: [],
    features: [],
    languages: [],
    proficiencies: [],
    equipment: [],
    coins: { cp: 0, sp: 0, ep: 0, gp: 0, pp: 0 },
    customFeaturesText: '',
    issues: [],
    hints: [],
    contentVersion: 'srd51@test',
    ...overrides,
  };
}

function basicSheet(overrides: Partial<BasicSheetVm> = {}): BasicSheetVm {
  return {
    kind: 'basic',
    hitPointsMax: 7,
    armorClass: 12,
    speedWalkFt: 30,
    attackBonus: 3,
    damage: '1d6+1 perfurante',
    description: 'Um goblin arisco.',
    ...overrides,
  };
}

function vm(overrides: Partial<CharacterSheetVm> = {}): CharacterSheetVm {
  return {
    id: 'char-1',
    campaignId: 'camp-1',
    characterKind: 'player',
    name: 'Pensantus',
    state: 'draft',
    canEdit: true,
    sheetLockedAt: null,
    diedAt: null,
    revision: 1,
    sheet: fullSheet(),
    story: emptyStory(),
    canEditStory: true,
    storyEditingAllowed: false,
    canToggleStoryEditing: false,
    canMarkDead: false,
    canAccessMasterNotes: false,
    canApprove: false,
    isMaster: false,
    playerDisplayName: 'Vinicius',
    raceLabel: 'Gnomo da Rocha',
    classSummary: 'Mago 3',
    backgroundLabel: 'Sábio',
    alignmentLabel: 'Neutro e bom',
    experiencePoints: 2700,
    ...overrides,
  };
}

function activatedRouteFor(campaignId: string, characterId: string) {
  return { paramMap: of(convertToParamMap({ id: campaignId, characterId })) };
}

function flush(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

describe('CharacterSheetPage', () => {
  let fake: FakeCharacterSheetSource;

  function configure(campaignId = 'camp-1', characterId = 'char-1'): void {
    TestBed.configureTestingModule({
      imports: [CharacterSheetPage],
      providers: [
        { provide: CharacterSheetSource, useClass: FakeCharacterSheetSource },
        { provide: ActivatedRoute, useValue: activatedRouteFor(campaignId, characterId) },
      ],
    });
    fake = TestBed.inject(CharacterSheetSource) as unknown as FakeCharacterSheetSource;
  }

  async function render(): Promise<HTMLElement> {
    const fixture = TestBed.createComponent(CharacterSheetPage);
    fixture.detectChanges();
    await flush();
    await fixture.whenStable();
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('shows exactly the server-sent numbers, even an inconsistent modifier', async () => {
    configure();
    fake.getCharacterSheetFn = () =>
      Promise.resolve(
        vm({
          sheet: fullSheet({
            abilities: [
              { key: 'int', score: 18, modifier: 9 },
              { key: 'str', score: 10, modifier: 0 },
              { key: 'dex', score: 10, modifier: 0 },
              { key: 'con', score: 10, modifier: 0 },
              { key: 'wis', score: 10, modifier: 0 },
              { key: 'cha', score: 10, modifier: 0 },
            ],
          }),
        }),
      );

    const el = await render();
    expect(el.textContent).toContain('18 (+9)');
  });

  it('shows a weapon attack with its bonus, and a damage cantrip with its save DC', async () => {
    configure();
    fake.getCharacterSheetFn = () =>
      Promise.resolve(
        vm({
          sheet: fullSheet({
            attacks: [
              {
                key: 'equipment:quarterstaff',
                namePt: 'Bordão',
                kind: 'weapon',
                attackBonus: 5,
                damage: '1d6+3',
                damageTypePt: 'concussão',
                saveDc: 0,
                saveAbility: null,
              },
              {
                key: 'spell:fire-bolt',
                namePt: 'Raio de Fogo',
                kind: 'spell',
                attackBonus: 6,
                damage: '1d10',
                damageTypePt: 'fogo',
                saveDc: 0,
                saveAbility: null,
              },
              {
                key: 'spell:poison-spray',
                namePt: 'Borrifo Venenoso',
                kind: 'spell',
                attackBonus: 0,
                damage: '1d12',
                damageTypePt: 'veneno',
                saveDc: 14,
                saveAbility: 'con',
              },
            ],
          }),
        }),
      );

    const el = await render();
    expect(el.textContent).toContain('Bordão');
    expect(el.textContent).toContain('+5');
    expect(el.textContent).toContain('1d6+3 concussão');
    expect(el.textContent).toContain('Raio de Fogo');
    expect(el.textContent).toContain('+6');
    expect(el.textContent).toContain('Borrifo Venenoso');
    expect(el.textContent).toContain('CD 14');
    expect(el.textContent).toContain('Constituição');
    expect(el.textContent).toContain('1d12 veneno');
  });

  it('shows spell slots as a readable, separated list using "círculo" (integrator fix)', async () => {
    configure();
    fake.getCharacterSheetFn = () =>
      Promise.resolve(
        vm({
          sheet: fullSheet({
            spellSlots: [4, 2],
            spellcasting: [
              {
                className: 'Mago',
                ability: 'int',
                saveDc: 14,
                attackBonus: 6,
                cantripsKnown: 3,
                spellsPreparedMax: 7,
              },
            ],
          }),
        }),
      );

    const el = await render();
    expect(el.textContent).toContain('1º círculo: 4 · 2º círculo: 2');
    // The old bug: two <span>s with nothing between them rendered as
    // "1º nível: 42º nível: 2" — no separator, and the wrong term.
    expect(el.textContent).not.toContain('42º');
    expect(el.textContent).not.toContain('nível: 4');
  });

  it('renders every official-sheet section as an <h2>, grouped by desktop column (integrator fix: three independent columns, no shared grid rows)', async () => {
    configure();
    fake.getCharacterSheetFn = () =>
      Promise.resolve(
        vm({
          sheet: fullSheet({
            spellcasting: [
              {
                className: 'Mago',
                ability: 'int',
                saveDc: 14,
                attackBonus: 6,
                cantripsKnown: 3,
                spellsPreparedMax: 7,
              },
            ],
          }),
        }),
      );

    const el = await render();
    const headings = Array.from(el.querySelectorAll('h2')).map((h) => h.textContent?.trim());
    // Document order follows the desktop column grouping (column 1:
    // Atributos, Salvaguardas, Perícias; column 2: Combate, Magias,
    // Equipamento; column 3: Características e traços, História) — a
    // sensible reading order on its own. Mobile reflows the same markup
    // into the agreed visual order (Atributos, Salvaguardas, Combate,
    // Perícias, Magias, Equipamento, Características e traços, História)
    // purely with CSS `order` (`character-sheet.scss`), which a unit test
    // running in jsdom (no layout engine) cannot observe.
    expect(headings).toEqual([
      'Atributos',
      'Salvaguardas',
      'Perícias',
      'Combate',
      'Magias',
      'Equipamento',
      'Características e traços',
      'História',
    ]);
  });

  it('shows the six saving throws in their own "Salvaguardas" section, proficiency marked like skills', async () => {
    configure();
    fake.getCharacterSheetFn = () =>
      Promise.resolve(
        vm({
          sheet: fullSheet({
            savingThrows: [
              { key: 'str', bonus: 5, proficient: true },
              { key: 'dex', bonus: 0, proficient: false },
              { key: 'con', bonus: 1, proficient: false },
              { key: 'int', bonus: 6, proficient: true },
              { key: 'wis', bonus: 1, proficient: false },
              { key: 'cha', bonus: 0, proficient: false },
            ],
          }),
        }),
      );

    const el = await render();
    const heading = Array.from(el.querySelectorAll('h2')).find(
      (h) => h.textContent?.trim() === 'Salvaguardas',
    );
    expect(heading).toBeTruthy();
    const section = heading!.closest('section')!;
    expect(section.textContent).toContain('Força: +5');
    expect(section.textContent).toContain('proficiente');
    expect(section.textContent).toContain('Destreza: +0');
    expect(section.textContent).toContain('sem proficiência');
    // No longer buried, unlabelled, at the top of Perícias.
    const pericias = Array.from(el.querySelectorAll('h2')).find(
      (h) => h.textContent?.trim() === 'Perícias',
    );
    expect(pericias!.closest('section')!.querySelector('.saves-list')).toBeNull();
  });

  it('shows the locked banner and hides "Editar ficha" when the player cannot edit', async () => {
    configure();
    fake.getCharacterSheetFn = () =>
      Promise.resolve(
        vm({
          state: 'locked',
          canEdit: false,
          sheetLockedAt: new Date('2026-09-29T12:00:00Z'),
        }),
      );

    const el = await render();
    expect(el.textContent).toContain('Ficha travada desde');
    expect(el.textContent).not.toContain('Editar ficha');
  });

  it('shows "Editar ficha" for the master even when the sheet is locked', async () => {
    configure();
    fake.getCharacterSheetFn = () =>
      Promise.resolve(
        vm({
          state: 'locked',
          canEdit: true,
          isMaster: true,
          canAccessMasterNotes: true,
          sheetLockedAt: new Date('2026-09-29T12:00:00Z'),
        }),
      );
    fake.getMasterNotesFn = () => Promise.resolve('');

    const el = await render();
    expect(el.textContent).toContain('Editar ficha');
  });

  it('shows the master notes panel and "Marcar como morto" only for the master', async () => {
    configure();
    fake.getCharacterSheetFn = () => Promise.resolve(vm({ isMaster: false }));

    const el = await render();
    expect(el.textContent).not.toContain('Notas do mestre');
    expect(el.textContent).not.toContain('Marcar como morto');
    // RN-11: the notes RPC is never even called for a player.
    expect(fake.getMasterNotesCalls).toEqual([]);
  });

  it('loads and shows the master notes panel for the master, never for a player', async () => {
    configure();
    fake.getCharacterSheetFn = () =>
      Promise.resolve(vm({ isMaster: true, canAccessMasterNotes: true, canMarkDead: true }));
    fake.getMasterNotesFn = () => Promise.resolve('Esconde um segredo.');

    const el = await render();
    expect(el.textContent).toContain('Notas do mestre');
    expect(el.textContent).toContain('Marcar como morto');
    expect(fake.getMasterNotesCalls).toEqual(['char-1']);
  });

  it('hides "Editar história" for a player once the sheet has locked it', async () => {
    configure();
    fake.getCharacterSheetFn = () =>
      Promise.resolve(vm({ isMaster: false, state: 'locked', canEdit: false, canEditStory: false }));

    const el = await render();
    expect(el.textContent).not.toContain('Editar história');
  });

  it('shows "Editar história" for a player once the master has unlocked it', async () => {
    configure();
    fake.getCharacterSheetFn = () =>
      Promise.resolve(vm({ isMaster: false, state: 'locked', canEdit: false, canEditStory: true }));

    const el = await render();
    expect(el.textContent).toContain('Editar história');
  });

  it('shows the master\'s story toggle, labeled by storyEditingAllowed, and flips it on click', async () => {
    configure();
    fake.getCharacterSheetFn = () =>
      Promise.resolve(
        vm({
          isMaster: true,
          canAccessMasterNotes: true,
          canToggleStoryEditing: true,
          storyEditingAllowed: false,
        }),
      );
    fake.getMasterNotesFn = () => Promise.resolve('');
    fake.setStoryEditingAllowedFn = () =>
      Promise.resolve(
        vm({
          isMaster: true,
          canToggleStoryEditing: true,
          storyEditingAllowed: true,
          // SetStoryEditing never changes the revision (integrator fix,
          // phase 2) — the response keeps it exactly as it was.
          revision: 1,
        }),
      );

    const el = await render();
    expect(el.textContent).toContain('Permitir editar a história');

    const toggle = Array.from(el.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('Permitir editar a história'),
    ) as HTMLButtonElement;
    toggle.click();
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(fake.setStoryEditingAllowedCalls).toEqual([{ id: 'char-1', allowed: true }]);
  });

  it('never shows the story toggle for a player', async () => {
    configure();
    fake.getCharacterSheetFn = () =>
      Promise.resolve(vm({ isMaster: false, canToggleStoryEditing: false }));

    const el = await render();
    expect(el.textContent).not.toContain('Permitir editar a história');
    expect(el.textContent).not.toContain('Travar a história');
  });

  it('renders an NPC basic sheet with its basic stats, and no ability grid', async () => {
    configure();
    fake.getCharacterSheetFn = () =>
      Promise.resolve(
        vm({
          characterKind: 'minion',
          playerDisplayName: null,
          story: null,
          sheet: basicSheet({ armorClass: 13, hitPointsMax: 7, attackBonus: 3, damage: '1d6+1 perfurante' }),
        }),
      );

    const el = await render();
    expect(el.textContent).toContain('13');
    expect(el.textContent).toContain('7');
    expect(el.textContent).toContain('1d6+1 perfurante');
    expect(el.querySelector('.ability-grid')).toBeNull();
  });

  it('shows the alignment and XP in the header, read from the stored sheet (integrator follow-up)', async () => {
    configure();
    fake.getCharacterSheetFn = () =>
      Promise.resolve(vm({ alignmentLabel: 'Caótico e bom', experiencePoints: 900 }));

    const el = await render();
    const meta = el.querySelector('.sheet-meta')?.textContent ?? '';
    expect(meta).toContain('Caótico e bom');
    expect(meta).toContain('XP: 900');
  });

  it('shows 0 XP (a real value), but hides alignment when it is unset', async () => {
    configure();
    fake.getCharacterSheetFn = () =>
      Promise.resolve(vm({ alignmentLabel: '', experiencePoints: 0 }));

    const el = await render();
    const meta = el.querySelector('.sheet-meta')?.textContent ?? '';
    expect(meta).toContain('XP: 0');
    expect(meta).not.toContain('Leal');
    expect(meta).not.toContain('Neutro');
    expect(meta).not.toContain('Caótico');
  });

  it('hides XP for an NPC basic sheet, which has none', async () => {
    configure();
    fake.getCharacterSheetFn = () =>
      Promise.resolve(
        vm({
          characterKind: 'minion',
          story: null,
          alignmentLabel: '',
          experiencePoints: null,
          sheet: basicSheet(),
        }),
      );

    const el = await render();
    const meta = el.querySelector('.sheet-meta')?.textContent ?? '';
    expect(meta).not.toContain('XP');
  });

  it('lists the armor, shield and weapons carried, not just free-text items (integrator fix)', async () => {
    configure();
    fake.getCharacterSheetFn = () =>
      Promise.resolve(
        vm({
          sheet: fullSheet({
            armorClassDescription: 'Armadura de couro + escudo',
            wearsArmor: true,
            hasShield: true,
            attacks: [
              {
                key: 'equipment:shortsword',
                namePt: 'Espada curta',
                kind: 'weapon',
                attackBonus: 4,
                damage: '1d6+2',
                damageTypePt: 'perfurante',
                saveDc: 0,
                saveAbility: null,
              },
              {
                key: 'spell:fire-bolt',
                namePt: 'Raio de Fogo',
                kind: 'spell',
                attackBonus: 6,
                damage: '1d10',
                damageTypePt: 'fogo',
                saveDc: 0,
                saveAbility: null,
              },
            ],
            equipment: [{ name: 'Corda (15m)', quantity: 1 }],
          }),
        }),
      );

    const el = await render();
    const heading = Array.from(el.querySelectorAll('h2')).find(
      (h) => h.textContent?.trim() === 'Equipamento',
    );
    const section = heading!.closest('section')!;
    expect(section.textContent).toContain('Armadura: Armadura de couro');
    expect(section.textContent).toContain('Escudo');
    expect(section.textContent).toContain('Espada curta');
    // A damage cantrip is not a weapon — never listed as equipment.
    expect(section.textContent).not.toContain('Raio de Fogo');
    expect(section.textContent).toContain('Corda (15m)');
    expect(section.textContent).not.toContain('Nenhum item cadastrado');
  });

  it('shows "Sem armadura" and no "Escudo" line when neither is carried', async () => {
    configure();
    fake.getCharacterSheetFn = () =>
      Promise.resolve(vm({ sheet: fullSheet({ armorClassDescription: 'Sem armadura' }) }));

    const el = await render();
    const heading = Array.from(el.querySelectorAll('h2')).find(
      (h) => h.textContent?.trim() === 'Equipamento',
    );
    const section = heading!.closest('section')!;
    expect(section.textContent).toContain('Armadura: Sem armadura');
    expect(section.textContent).not.toContain('Escudo');
  });

  it('says "Sem armadura" when a feature, not armor, gives the AC (Unarmored Defense)', async () => {
    configure();
    fake.getCharacterSheetFn = () =>
      Promise.resolve(vm({ sheet: fullSheet({ armorClassDescription: 'Defesa sem Armadura', wearsArmor: false }) }));

    const el = await render();
    const heading = Array.from(el.querySelectorAll('h2')).find(
      (h) => h.textContent?.trim() === 'Equipamento',
    );
    const section = heading!.closest('section')!;
    expect(section.textContent).toContain('Armadura: Sem armadura');
    expect(section.textContent).not.toContain('Armadura: Defesa sem Armadura');
  });

  it('shows each feature as a compact row, its English description collapsed by default, and lists issues and hints as "Avisos"', async () => {
    configure();
    fake.getCharacterSheetFn = () =>
      Promise.resolve(
        vm({
          sheet: fullSheet({
            features: [
              {
                name: 'Recuperação Arcana',
                sourcePt: 'Mago 1',
                description: 'You have learned to regain some of your magical energy.',
              },
            ],
            issues: [
              {
                code: 'unknown_key',
                field: 'full.armor_key',
                message: 'A armadura escolhida não existe no conteúdo srd51@test.',
              },
            ],
            hints: [
              {
                sourceKey: 'trait:gnome-cunning',
                text: 'Vantagem em testes de resistência de INT, SAB e CAR contra magia.',
              },
            ],
          }),
        }),
      );

    const el = await render();
    expect(el.textContent).toContain('Recuperação Arcana · Mago 1');

    const details = el.querySelector('details');
    expect(details).toBeTruthy();
    expect(details!.open).toBe(false);
    expect(details!.textContent).toContain(
      'You have learned to regain some of your magical energy.',
    );

    const avisosHeading = Array.from(el.querySelectorAll('h3')).find(
      (h) => h.textContent?.trim() === 'Avisos',
    );
    expect(avisosHeading).toBeTruthy();
    expect(el.textContent).toContain('A armadura escolhida não existe no conteúdo srd51@test.');
    expect(el.textContent).toContain(
      'Vantagem em testes de resistência de INT, SAB e CAR contra magia.',
    );
  });

  it('shows no "Avisos" heading when there are no issues or hints', async () => {
    configure();
    fake.getCharacterSheetFn = () => Promise.resolve(vm());

    const el = await render();
    const avisosHeading = Array.from(el.querySelectorAll('h3')).find(
      (h) => h.textContent?.trim() === 'Avisos',
    );
    expect(avisosHeading).toBeUndefined();
  });
});

describe('CharacterSheetPage: approval (MR-024)', () => {
  let fake: FakeCharacterSheetSource;

  beforeEach(() => {
    TestBed.configureTestingModule({
      imports: [CharacterSheetPage],
      providers: [
        { provide: CharacterSheetSource, useClass: FakeCharacterSheetSource },
        { provide: ActivatedRoute, useValue: activatedRouteFor('camp-1', 'char-1') },
      ],
    });
    fake = TestBed.inject(CharacterSheetSource) as unknown as FakeCharacterSheetSource;
  });

  const pendingForMaster = () =>
    vm({ state: 'pending', canApprove: true, isMaster: true, canAccessMasterNotes: true });

  async function render() {
    const fixture = TestBed.createComponent(CharacterSheetPage);
    fixture.detectChanges();
    await flush();
    await fixture.whenStable();
    fixture.detectChanges();
    return fixture;
  }

  function button(el: HTMLElement, text: string): HTMLButtonElement | undefined {
    return Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.trim() === text);
  }

  it('the pending player sees "Esperando a aprovação do mestre", and no approval buttons', async () => {
    fake.getCharacterSheetFn = () => Promise.resolve(vm({ state: 'pending', canApprove: false }));
    const el = (await render()).nativeElement as HTMLElement;

    expect(el.textContent).toContain('Pendente de aprovação');
    expect(el.textContent).toContain('Esperando a aprovação do mestre');
    expect(button(el, 'Aprovar personagem')).toBeUndefined();
    expect(button(el, 'Recusar personagem')).toBeUndefined();
    // Still editable while waiting.
    expect(el.textContent).toContain('Editar ficha');
  });

  it('"Aprovar personagem" approves and shows the character as a draft', async () => {
    fake.getCharacterSheetFn = () => Promise.resolve(pendingForMaster());
    fake.approveCharacterFn = () =>
      Promise.resolve(vm({ state: 'draft', canApprove: false, isMaster: true, canAccessMasterNotes: true }));
    const fixture = await render();
    const el = fixture.nativeElement as HTMLElement;

    button(el, 'Aprovar personagem')!.click();
    await flush();
    fixture.detectChanges();

    expect(el.textContent).toContain('Rascunho');
    expect(button(el, 'Aprovar personagem')).toBeUndefined();
  });

  it('"Recusar personagem" asks to confirm, then rejects and goes back to the campaign', async () => {
    fake.getCharacterSheetFn = () => Promise.resolve(pendingForMaster());
    fake.rejectCharacterFn = () => Promise.resolve();
    const navigate = vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
    const fixture = await render();
    const el = fixture.nativeElement as HTMLElement;

    button(el, 'Recusar personagem')!.click();
    fixture.detectChanges();
    expect(fake.rejectCharacterCalls).toEqual([]); // nothing yet: one more click
    expect(button(el, 'Cancelar')).toBeTruthy();

    button(el, 'Confirmar recusa')!.click();
    await flush();
    fixture.detectChanges();

    expect(fake.rejectCharacterCalls).toEqual(['char-1']);
    expect(navigate).toHaveBeenCalledWith(['/campanhas', 'camp-1']);
  });

  it('shows the server\'s reason when the rejection fails', async () => {
    fake.getCharacterSheetFn = () => Promise.resolve(pendingForMaster());
    fake.rejectCharacterFn = () => Promise.reject(new ConnectError('gone', Code.NotFound));
    const fixture = await render();
    const el = fixture.nativeElement as HTMLElement;

    button(el, 'Recusar personagem')!.click();
    fixture.detectChanges();
    button(el, 'Confirmar recusa')!.click();
    await flush();
    fixture.detectChanges();

    expect(el.querySelector('[role="alert"]')?.textContent).toContain('Personagem não encontrado');
  });
});
