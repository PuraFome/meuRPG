import { NpcCheckSource } from '../../../gen/meurpg/play/v1/scene_pb';
import { NpcCheckForm } from './npc-check-form';
import { candidate } from './npc-check-testing';

const capitao = candidate({ characterId: 'npc-cb', monsterKey: '', name: 'Capitão bandido' });
const bartolo = candidate({
  characterId: 'npc-bartolo',
  monsterKey: '',
  name: 'Bartolo',
  hasStatBlock: false,
  checks: [{ key: 'skill:persuasion', name: 'Persuasão', bonus: 0, known: false }],
});
const espiao = candidate({
  characterId: '',
  monsterKey: 'spy',
  name: 'Espião',
  source: NpcCheckSource.TOKEN,
  hidden: true,
  mapMeasures: false,
  checks: [{ key: 'skill:stealth', name: 'Furtividade', bonus: 4, known: true }],
});
const medido = candidate({
  characterId: 'npc-m',
  monsterKey: '',
  name: 'Guarda',
  mapMeasures: true,
});

function formWith(...list: ReturnType<typeof candidate>[]): NpcCheckForm {
  const form = new NpcCheckForm();
  form.setCandidates(list);
  return form;
}

describe('NpcCheckForm', () => {
  it('starts with nobody chosen: the roll is not ready and says what is missing', () => {
    const form = formWith(capitao, bartolo);
    expect(form.selected()).toBeNull();
    expect(form.ready()).toBe(false);
    expect(form.reason()).toBe('Escolha quem rola e o teste.');
    expect(form.rollLabel()).toBe('Rolar o teste');
    expect(form.roll()).toBeNull();
  });

  it('chooses the creature, starts on Deception and names the roll', () => {
    const form = formWith(capitao, bartolo);
    form.select('npc-cb');
    expect(form.checkKey()).toBe('skill:deception');
    expect(form.ready()).toBe(true);
    expect(form.reason()).toBe('');
    expect(form.rollLabel()).toBe('Rolar Enganação de Capitão bandido');
  });

  it('says a creature with no number for the check rolls +0, beside the check', () => {
    const form = formWith(capitao, bartolo);
    form.select('npc-bartolo');
    expect(form.noBonus()).toBe(
      'Sem bônus para Persuasão: o d20 conta com +0 e você soma o bônus que achar justo.',
    );
    expect(form.options()[0].side).toBe('sem bônus');
    form.select('npc-cb');
    expect(form.noBonus()).toBe('');
  });

  it('starts the box ticked for a creature the players see and unticked for a hidden one', () => {
    const form = formWith(capitao, espiao);
    form.select('npc-cb');
    expect(form.showsBox()).toBe(true);
    expect(form.box()).toBe(true);
    form.select('spy');
    expect(form.box()).toBe(false);
    form.setBox(true);
    expect(form.box()).toBe(true);
    form.select('npc-cb');
    expect(form.box()).toBe(true);
    form.select('spy');
    expect(form.box()).toBe(false);
  });

  it('shows no box when the map measures, and sends bardSees false', () => {
    const form = formWith(medido);
    form.select('npc-m');
    expect(form.showsBox()).toBe(false);
    expect(form.roll()?.bardSees).toBe(false);
  });

  it('rolls in the app, or with the typed d20 from 1 to 20', () => {
    const form = formWith(capitao);
    form.select('npc-cb');
    expect(form.roll()).toEqual({
      characterId: 'npc-cb',
      monsterKey: '',
      checkKey: 'skill:deception',
      die: { inApp: true },
      bardSees: true,
    });
    form.setMode('type');
    expect(form.ready()).toBe(false);
    expect(form.reason()).toBe('O d20 vai de 1 a 20.');
    form.setText('21');
    expect(form.ready()).toBe(false);
    form.setText(' 11 ');
    expect(form.ready()).toBe(true);
    expect(form.roll()?.die).toEqual({ face: 11 });
  });

  it("asks nothing of the d20 field while the dice are the app's", () => {
    const form = formWith(capitao);
    form.select('npc-cb');
    form.setText('77');
    expect(form.ready()).toBe(true);
    expect(form.invalid()).toBe(false);
    form.setMode('type');
    expect(form.invalid()).toBe(true);
    form.setText('');
    expect(form.invalid()).toBe(false);
  });

  it('adds the creature the bestiary found in its group and chooses it', () => {
    const form = formWith(capitao);
    const found = candidate({
      characterId: '',
      monsterKey: 'goblin',
      name: 'Goblin',
      source: NpcCheckSource.MONSTER,
    });
    form.addFound(found);
    expect(
      form
        .groups()
        .at(-1)
        ?.rows.map((r) => r.name),
    ).toEqual(['Goblin']);
    expect(form.selected()?.monsterKey).toBe('goblin');
    const other = candidate({
      characterId: '',
      monsterKey: 'orc',
      name: 'Orc',
      source: NpcCheckSource.MONSTER,
    });
    form.addFound(other);
    expect(
      form
        .groups()
        .at(-1)
        ?.rows.map((r) => r.name),
    ).toEqual(['Orc']);
  });

  it('keeps the choice when the list is read again and drops it when the creature is gone', () => {
    const form = formWith(capitao, bartolo);
    form.select('npc-bartolo');
    form.setCandidates([bartolo]);
    expect(form.selected()?.name).toBe('Bartolo');
    form.setCandidates([capitao]);
    expect(form.selected()).toBeNull();
  });
});
