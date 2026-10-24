import { EffectPhase } from '../../../../../gen/meurpg/play/v1/lasting_effects_pb';
import { reactionWindow } from '../../../../core/combat/combat-testing';
import {
  bonusWord,
  concentrationBonus,
  concentrationDc,
  concentrationDice,
  concentrationLead,
  concentrationMode,
  concentrationModeLine,
  concentrationRule,
  concentrationSaid,
  concentrationTitle,
  effectSaveAbility,
  effectSaveBonus,
  effectSaveMode,
  effectSavePairHint,
  effectSaveRule,
  effectSaveSaid,
  effectSaveSkippable,
  effectSaveText,
  effectSaveTitle,
  effectSaveWho,
  rebukeAggressor,
  rebukeBonus,
  rebukeModifier,
  rebukeText,
  rebukeTitle,
  sayAnswer,
  signed,
} from './reaction-queue-text';

const win = (case_: string | undefined, value: object | undefined, over: object = {}) =>
  reactionWindow({
    id: 'w',
    reactorLabel: 'Goblin',
    reactorIsPlayer: false,
    prompt: { case: case_, value } as never,
    ...over,
  } as never);
const bare = () => win(undefined, undefined);
const save = (value: object = {}, over: object = {}) =>
  win(
    'effectSave',
    {
      sourceNamePt: 'Riso Histérico',
      abilityNamePt: 'Sabedoria',
      dc: 13,
      modifier: -1,
      bonusKnown: true,
      mode: '',
      textPt: 'Role.',
      skippable: false,
      phase: EffectPhase.END,
      ...value,
    },
    over,
  );
const rebuke = (value: object = {}, over: object = {}) =>
  win(
    'hellishRebukeSave',
    {
      aggressorLabel: 'Hobgoblin',
      saveDc: 13,
      saveBonus: 1,
      bonusKnown: true,
      diceCount: 3,
      ...value,
    },
    over,
  );
const conc = (value: object = {}, over: object = {}) =>
  win(
    'concentrationSave',
    {
      spellNamePt: 'Voo',
      damageTaken: 9,
      dc: 12,
      saveBonus: 2,
      bonusKnown: true,
      mode: '',
      modeSourcesPt: [],
      ...value,
    },
    { trigger: { damageTaken: 9, concentrationSpellKey: 'spell:fly' }, ...over },
  );

describe('reaction-queue-text', () => {
  it('signs a number with the true minus', () => {
    expect(signed(3)).toBe('+3');
    expect(signed(0)).toBe('+0');
    expect(signed(-2)).toBe('−2');
  });

  describe("an effect's saving throw", () => {
    it('reads the mode, and the hint of the pair says which die counts', () => {
      expect(effectSaveMode(save({ mode: 'advantage' }))).toBe('advantage');
      expect(effectSaveMode(save({ mode: 'disadvantage' }))).toBe('disadvantage');
      expect(effectSaveMode(save({ mode: 'other' }))).toBe('normal');
      expect(effectSaveMode(bare())).toBe('normal');
      expect(effectSavePairHint(save({ mode: 'advantage' }))).toContain(
        'Com vantagem conta o maior',
      );
      expect(effectSavePairHint(save({ mode: 'disadvantage' }))).toContain(
        'Com desvantagem conta o menor',
      );
    });

    it('titles the save with the ability, whose it is, the source and the DC', () => {
      expect(effectSaveTitle(save())).toBe(
        'Teste de resistência de Sabedoria do Goblin · Riso Histérico · CD 13',
      );
      expect(effectSaveTitle(save({}, { reactorIsPlayer: true, reactorLabel: 'Brisa' }))).toBe(
        'Teste de resistência de Sabedoria de Brisa · Riso Histérico · CD 13',
      );
      expect(effectSaveTitle(save({ sourceNamePt: '', dc: undefined }))).toBe(
        'Teste de resistência de Sabedoria do Goblin',
      );
      expect(effectSaveTitle(bare())).toBe('Teste de resistência de  do Goblin');
    });

    it('names the ability from its key when the server sends the key and an empty name (a zone)', () => {
      const zone = save({ zoneId: 'z1', ability: 'con', abilityNamePt: '', dc: 14 });
      expect(effectSaveAbility(zone)).toBe('Constituição');
      expect(effectSaveTitle(zone, 'Bartolo')).toBe(
        'Riso Histérico de Bartolo · Teste de resistência de Constituição do Goblin · CD\u00a014',
      );
      expect(effectSaveRule(save({ ability: 'con', abilityNamePt: '' }))).toBe('Constituição: −1');
    });

    it('says when the effect asks for it', () => {
      expect(effectSaveText(save())).toBe(
        'Riso Histérico pede este teste no fim da vez do Goblin. Role.',
      );
      expect(effectSaveText(save({ phase: EffectPhase.START, textPt: '' }))).toBe(
        'Riso Histérico pede este teste no começo da vez do Goblin.',
      );
      expect(effectSaveText(bare())).toBe('Um efeito pede este teste no fim da vez do Goblin.');
    });

    it('writes the rule with the bonus, or says it is unknown', () => {
      expect(effectSaveRule(save({ mode: 'advantage' }))).toBe('Sabedoria: −1 · advantage');
      expect(effectSaveRule(save({ bonusKnown: false }))).toBe('Bônus desconhecido');
      expect(effectSaveRule(bare())).toBe('');
    });

    it('names the ability, the bonus, who left the roll and whether it can be skipped', () => {
      expect(effectSaveAbility(save())).toBe('Sabedoria');
      expect(effectSaveAbility(bare())).toBe('');
      expect(effectSaveBonus(save())).toBe(-1);
      expect(effectSaveBonus(save({ bonusKnown: false }))).toBe(0);
      expect(effectSaveBonus(bare())).toBe(0);
      expect(effectSaveWho(save())).toBe('');
      expect(effectSaveWho(save({}, { reactorIsPlayer: true, reactorLabel: 'Brisa' }))).toBe(
        'O jogador de Brisa deixou a rolagem com você.',
      );
      expect(effectSaveSkippable(save({ skippable: true }))).toBe(true);
      expect(effectSaveSkippable(save())).toBe(false);
      expect(effectSaveSkippable(bare())).toBe(false);
    });

    it('says what was done', () => {
      expect(effectSaveSaid(save(), { kind: 'app' })).toBe(
        'Teste de resistência do Goblin resolvido.',
      );
      expect(effectSaveSaid(save(), { kind: 'skip' })).toBe('Teste pulado do Goblin.');
    });
  });

  describe("Repreensão Infernal's saving throw", () => {
    it('titles it for the player or the NPC', () => {
      expect(rebukeTitle(rebuke({}, { reactorIsPlayer: true, reactorLabel: 'Mirta' }))).toBe(
        'Repreensão Infernal de Mirta',
      );
      expect(rebukeTitle(rebuke())).toBe('Repreensão Infernal do Goblin');
    });

    it('says the damage and the bonus of the aggressor', () => {
      expect(rebukeAggressor(rebuke())).toBe('Hobgoblin');
      expect(rebukeText(rebuke())).toBe(
        'Hobgoblin sofre 3d10 de fogo, ou metade se passar no teste de Destreza contra CD 13.',
      );
      expect(rebukeBonus(rebuke())).toBe('Destreza do Hobgoblin: +1 · o teste é seu');
      expect(rebukeBonus(rebuke({ bonusKnown: false }))).toBe(
        'Destreza do Hobgoblin: desconhecida · o teste é seu',
      );
      expect(rebukeModifier(rebuke())).toBe(1);
      expect(rebukeModifier(rebuke({ bonusKnown: false }))).toBe(0);
    });

    it('has nothing to say for a window that is not one', () => {
      expect(rebukeAggressor(bare())).toBe('');
      expect(rebukeText(bare())).toBe('');
      expect(rebukeBonus(bare())).toBe('');
      expect(rebukeModifier(bare())).toBe(0);
    });
  });

  describe('the concentration save', () => {
    it('says who took the damage, the spell and the rule with the half of it', () => {
      expect(concentrationTitle(conc())).toBe('Goblin sofreu 9 de dano');
      expect(concentrationTitle(bare())).toBe('Goblin sofreu 0 de dano');
      expect(concentrationLead(conc())).toBe('Goblin concentra em Voo.');
      expect(concentrationLead(bare())).toBe('Goblin concentra em .');
      expect(
        concentrationLead(
          win(undefined, undefined, { trigger: { concentrationSpellKey: 'spell:fly' } }),
        ),
      ).toBe('Goblin concentra em spell:fly.');
      expect(concentrationDc(conc())).toBe(12);
      expect(concentrationDc(bare())).toBe(10);
      expect(concentrationRule(conc())).toBe(
        'Teste de resistência de Constituição contra CD 12 (metade de 9 seria 4; o mínimo é 10) · Constituição do Goblin: +2',
      );
      expect(concentrationRule(conc({ bonusKnown: false }))).toBe(
        'Teste de resistência de Constituição contra CD 12 (metade de 9 seria 4; o mínimo é 10)',
      );
    });

    it('reads the mode, how many d20 it takes and the line about it', () => {
      expect(concentrationMode(conc({ mode: 'advantage' }))).toBe('advantage');
      expect(concentrationMode(bare())).toBe('normal');
      expect(concentrationDice(conc())).toBe(1);
      expect(concentrationDice(conc({ mode: 'disadvantage' }))).toBe(2);
      expect(concentrationModeLine(conc())).toBe('');
      expect(concentrationModeLine(bare())).toBe('');
      expect(concentrationModeLine(conc({ mode: 'advantage', modeSourcesPt: ['Bênção'] }))).toBe(
        'Goblin tem vantagem neste teste (Bênção). Role dois d20; vale o maior.',
      );
    });

    it('writes the bonus with its sign', () => {
      expect(concentrationBonus(conc({ saveBonus: -2 }))).toBe(-2);
      expect(concentrationBonus(conc({ bonusKnown: false }))).toBe(0);
      expect(concentrationBonus(bare())).toBe(0);
      expect(bonusWord(conc({ saveBonus: -2 }))).toBe('− 2');
      expect(bonusWord(conc())).toBe('+ 2');
      expect(bonusWord(bare())).toBe('+ 0');
    });

    it('says the result, or that it was kept, or only that it was answered', () => {
      const w = conc();
      const lost = { dc: 12, kept: false, spellNamePt: 'Voo' } as never;
      expect(concentrationSaid(w, { kind: 'app' }, lost)).toBe(
        'contra CD 12: Goblin perdeu a concentração em Voo.',
      );
      expect(
        concentrationSaid(w, { kind: 'app' }, { dc: 12, kept: true, spellNamePt: '' } as never),
      ).toBe('contra CD 12: Goblin manteve a concentração.');
      expect(
        concentrationSaid(w, { kind: 'app' }, {
          dc: 12,
          kept: true,
          spellNamePt: '',
          save: { physical: true, total: 14, modifier: 0 },
        } as never),
      ).toBe('14 · dado físico contra CD 12: Goblin manteve a concentração.');
      expect(concentrationSaid(w, { kind: 'keep' }, undefined)).toBe(
        'Concentração mantida do Goblin.',
      );
      expect(concentrationSaid(w, { kind: 'app' }, undefined)).toBe('Respondido.');
    });
  });

  describe('a reaction answered', () => {
    it('says a pass, a shield that stopped the hit or not, and any other use', () => {
      const w = win('shield', {});
      expect(sayAnswer(w, { use: false }, undefined)).toBe('Deixou passar do Goblin.');
      expect(sayAnswer(w, { use: true }, { case: 'shield', value: { stopped: true } })).toBe(
        'Escudo Arcano usado: o ataque errou.',
      );
      expect(sayAnswer(w, { use: true }, { case: 'shield', value: { stopped: false } })).toBe(
        'Escudo Arcano usado: o ataque ainda acerta.',
      );
      expect(sayAnswer(w, { use: true }, undefined)).toBe('Reação usada.');
      expect(sayAnswer(w, { use: true }, { case: 'counterspell', value: {} })).toBe(
        'Reação usada.',
      );
    });
  });
});

describe('the master queue for a zone save', () => {
  const zoneSave = (over: object = {}) =>
    save(
      {
        zoneId: 'z1',
        zoneReason: 'zone_enter',
        sourceNamePt: 'Teia',
        phase: EffectPhase.UNSPECIFIED,
        ...over,
      },
      { reactorLabel: 'Tor', reactorIsPlayer: true },
    );

  it('says why the zone asks, not an end or start of the turn', () => {
    expect(effectSaveText(zoneSave())).toContain('Tor entrou na zona.');
    expect(effectSaveText(zoneSave({ zoneReason: 'zone_start' }))).toContain(
      'A zona atingiu Tor no começo do turno.',
    );
    expect(effectSaveText(zoneSave())).not.toContain('no fim da vez');
  });

  it('titles it with the zone and the DC he reads, and the caster when he has it', () => {
    expect(effectSaveTitle(zoneSave(), 'Pensantus')).toBe(
      'Teia de Pensantus · Teste de resistência de Sabedoria de Tor · CD\u00a013',
    );
    expect(effectSaveTitle(save())).toBe(
      'Teste de resistência de Sabedoria do Goblin · Riso Histérico · CD 13',
    );
  });
});
