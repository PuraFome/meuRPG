import { Injectable } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { Code, ConnectError } from '@connectrpc/connect';
import { of } from 'rxjs';

import {
  CharacterBlockedReason,
  CharacterBlockedSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { CharacterEditor } from './character-editor';
import {
  CharacterEditorSource,
  CharacterForEdit,
  CreateCharacterInput,
  RulesCatalogVm,
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

function catalog(): RulesCatalogVm {
  return {
    races: [
      {
        key: 'race:gnome',
        namePt: 'Gnomo',
        subraces: [{ key: 'subrace:rock-gnome', namePt: 'Gnomo da Rocha' }],
      },
    ],
    classes: [
      {
        key: 'class:wizard',
        namePt: 'Mago',
        isCaster: true,
        preparation: 'spellbook',
        subclasses: [{ key: 'subclass:evocation', namePt: 'Evocação' }],
      },
    ],
    backgrounds: [{ key: 'background:acolyte', namePt: 'Acólito' }],
    skills: [
      { key: 'skill:arcana', namePt: 'Arcanismo', ability: 'int' },
      { key: 'skill:history', namePt: 'História', ability: 'int' },
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

  async function render(): Promise<{ fixture: ComponentFixture<CharacterEditor>; el: HTMLElement }> {
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
      cantripsText: 'Fire Bolt\nRay of Frost',
      equipmentText: 'Grimório\nAdaga',
    });
    cmp.selectedSkills.set(new Set(['skill:arcana', 'skill:history']));

    await cmp.submit();

    expect(fake.createCharacterCalls.length).toBe(1);
    const req = fake.createCharacterCalls[0];
    expect(req.campaignId).toBe('camp-1');
    expect(req.kind).toBe('player');
    expect(req.full?.name).toBe('Pensantus');
    expect(req.full?.race).toBe('race:gnome');
    expect(req.full?.level).toBe(3);
    expect(req.full?.skillProficiencies.sort()).toEqual(['skill:arcana', 'skill:history']);
    expect(req.full?.cantrips).toEqual(['Fire Bolt', 'Ray of Frost']);
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

  it('shows only the spell lists that match the class\'s preparation style', async () => {
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
    expect(req.full?.extraAbilityBonuses).toEqual({ str: 0, dex: 0, con: 1, int: 2, wis: 0, cha: 0 });
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
});
