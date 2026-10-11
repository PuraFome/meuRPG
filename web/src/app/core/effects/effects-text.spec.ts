import {
  EffectAudience,
  EffectDurationKind,
  EffectPhase,
} from '../../../gen/meurpg/play/v1/lasting_effects_pb';
import {
  EXHAUSTION_LEVELS,
  TIME_PRESETS,
  advanceDoneText,
  cardSubtitle,
  clockTitle,
  defaultDurationText,
  durationSpec,
  elapsedText,
  endConcentrationText,
  endLine,
  groupOf,
  lowerFirst,
  modeOfKind,
  namesList,
  panelSubtitle,
  saveLine,
  targetsText,
  uniqueNames,
  uniqueTargets,
  validDc,
  validRounds,
  validSeconds,
  visibilityText,
} from './effects-text';
import { boardEffects, catalogEffect, clockEntry, lastingEffect } from './effects-testing';

describe('effects text', () => {
  const [hold1, hold2, bless, dodge, prone] = boardEffects().effects;

  it('says the panel in one line', () => {
    expect(panelSubtitle(5, 3, 'Nael')).toBe('5 efeitos · rodada 3 · vez de Nael');
    expect(panelSubtitle(1, 0, '')).toBe('1 efeito');
  });

  it('names the targets, with their number when there are several', () => {
    expect(targetsText(bless)).toBe('Toren, Brisa, Ragna (3 alvos)');
    expect(targetsText(hold2)).toBe('Goblin 2');
    expect(namesList(['Toren', 'Brisa', 'Ragna'])).toBe('Toren, Brisa e Ragna');
    expect(namesList(['Toren'])).toBe('Toren');
  });

  it('tells what the players see, with the audience and the label', () => {
    expect(visibilityText(hold1)).toBe('Sim: Paralisada');
    expect(visibilityText(bless)).toBe('Sim');
    expect(visibilityText(lastingEffect({ id: 'x', playerVisible: false }))).toBe('Não');
    expect(
      visibilityText(
        lastingEffect({ id: 'x', audience: EffectAudience.OWNER, playerVisible: true }),
      ),
    ).toBe('Sim: só o dono do alvo');
  });

  it('writes the saving throw with the DC the master reads and whose turn it is', () => {
    expect(saveLine(hold1)).toBe('Teste de Sabedoria, CD 11, no fim do turno de Brisa.');
    expect(saveLine(bless)).toBe('');
    const several = lastingEffect({
      id: 'y',
      targetLabels: ['A', 'B'],
      targetIds: ['a', 'b'],
      startSave: { ability: 'dex', abilityNamePt: 'Destreza', phase: EffectPhase.START },
    });
    expect(saveLine(several)).toBe('Teste de Destreza, no começo do turno de cada alvo.');
  });

  it('keeps the server words for how an effect ends', () => {
    expect(endLine(dodge)).toContain('Até o começo do turno de Toren, rodada 4');
    expect(
      endLine(
        lastingEffect({ id: 'z', endTextPt: '', clockTextPt: 'Dura até o mestre encerrar.' }),
      ),
    ).toBe('Dura até o mestre encerrar.');
  });

  it('says whose it is on the phone card', () => {
    expect(cardSubtitle(hold2)).toBe('Em Goblin 2 · de Orla');
    expect(lowerFirst('Do mestre')).toBe('do mestre');
  });

  it('titles the clock by phase, and by turn for what runs out by rounds', () => {
    const effects = boardEffects().effects;
    const clock = boardEffects().turnClock;
    expect(clockTitle(clock[0], effects)).toBe('Fim do turno de Goblin 2 (rodada 3)');
    expect(clockTitle(clock[2], effects)).toBe('Começo do turno de Toren (rodada 4)');
    expect(clockTitle(clock[3], effects)).toBe('Turno de Tavo (rodada 11)');
    expect(
      clockTitle(clockEntry({ effectId: 'gone', phase: EffectPhase.START, isSave: false }), []),
    ).toBe('Começo do turno de Goblin 2 (rodada 3)');
  });

  it('asks before ending a concentration, saying what goes and from whom', () => {
    const group = groupOf(bless, boardEffects().effects);
    expect(endConcentrationText('Tavo', uniqueNames(group), uniqueTargets(group))).toBe(
      'A concentração de Tavo acaba e Bênção sai de Toren, Brisa e Ragna. Isto não se desfaz.',
    );
    expect(endConcentrationText('Tavo', ['Bênção', 'Perdição'], ['Toren'])).toBe(
      'A concentração de Tavo acaba e saem de Toren: Bênção e Perdição. Isto não se desfaz.',
    );
    expect(endConcentrationText('', ['Bênção'], ['Toren'])).toContain('A concentração acaba');
  });

  it('groups the effects of one casting', () => {
    const a = lastingEffect({ id: 'a', groupId: 'g' });
    const b = lastingEffect({ id: 'b', groupId: 'g' });
    const c = lastingEffect({ id: 'c', groupId: 'other' });
    expect(groupOf(a, [a, b, c]).map((e) => e.id)).toEqual(['a', 'b']);
    expect(groupOf(prone, [prone]).map((e) => e.id)).toEqual(['prone']);
  });

  it('describes the catalog default duration', () => {
    expect(defaultDurationText(catalogEffect({ key: 'spell:haste', concentration: true }))).toBe(
      '10 rodadas, concentração',
    );
    expect(
      defaultDurationText(
        catalogEffect({
          key: 'condition:prone',
          defaultDurationKind: EffectDurationKind.UNTIL_DISMISSED,
        }),
      ),
    ).toBe('Até o mestre encerrar');
  });

  it('builds the duration the server takes', () => {
    expect(durationSpec('rounds', 9, '', true)).toEqual({
      kind: EffectDurationKind.ROUNDS,
      rounds: 9,
      anchorCombatantId: undefined,
    });
    expect(durationSpec('turn', 0, 'orla', true)).toEqual({
      kind: EffectDurationKind.UNTIL_END_OF_TURN_OF,
      anchorCombatantId: 'orla',
    });
    expect(durationSpec('turn', 0, 'orla', false).kind).toBe(
      EffectDurationKind.UNTIL_START_OF_TURN_OF,
    );
    expect(durationSpec('dismissed', 0, '', true)).toEqual({
      kind: EffectDurationKind.UNTIL_DISMISSED,
    });
    expect(modeOfKind(EffectDurationKind.ROUNDS)).toBe('rounds');
    expect(modeOfKind(EffectDurationKind.UNTIL_END_OF_TURN_OF)).toBe('turn');
    expect(modeOfKind(EffectDurationKind.LONG_REST)).toBe('dismissed');
  });

  it('checks the limits the server keeps', () => {
    expect([0, 1, 600, 601, 1.5].map(validRounds)).toEqual([false, true, true, false, false]);
    expect([0, 1, 40, 41].map(validDc)).toEqual([false, true, true, false]);
    expect([0, 1, 86400, 86401].map(validSeconds)).toEqual([false, true, true, false]);
  });

  it('has the presets of the board and the six levels of exhaustion', () => {
    expect(TIME_PRESETS.map((p) => [p.label, p.seconds])).toEqual([
      ['1 rodada (6 s)', 6],
      ['1 minuto', 60],
      ['10 minutos', 600],
      ['1 hora', 3600],
    ]);
    expect(EXHAUSTION_LEVELS.map((l) => l.level)).toEqual([0, 1, 2, 3, 4, 5, 6]);
    expect(EXHAUSTION_LEVELS[4].text).toBe('PV máximos pela metade (e os de baixo)');
  });

  it('says how much time went by and what it ended', () => {
    expect(elapsedText(6)).toBe('6 segundos');
    expect(elapsedText(1)).toBe('1 segundo');
    expect(elapsedText(60)).toBe('1 minuto');
    expect(elapsedText(600)).toBe('10 minutos');
    expect(elapsedText(3600)).toBe('1 hora');
    expect(elapsedText(7200)).toBe('2 horas');
    expect(advanceDoneText(60, 0)).toBe('Passou 1 minuto. Nenhum efeito acabou.');
    expect(advanceDoneText(600, 1)).toBe('Passou 10 minutos. 1 efeito acabou.');
    expect(advanceDoneText(3600, 3)).toBe('Passou 1 hora. 3 efeitos acabaram.');
  });
});
