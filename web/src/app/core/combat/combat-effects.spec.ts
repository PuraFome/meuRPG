import {
  aidBar,
  aidLabel,
  aidRowLabel,
  aidTag,
  aidTagForMaster,
  armorClassWithShield,
  endAidText,
  sheetMaximum,
  shieldEndedForMaster,
  shieldEndedParts,
  shieldEndedSentence,
  shieldLabelForMaster,
  shieldLabelForOwner,
  shieldSum,
  vitalsSpeech,
} from './combat-effects';

describe('the Escudo Arcano and Ajuda in words (PM-03a)', () => {
  it('sums the shield into the armor class and writes the small sum', () => {
    expect(armorClassWithShield(13, 5)).toBe(18);
    expect(armorClassWithShield(13, 0)).toBe(13);
    expect(shieldSum(13, 5)).toBe('13 + 5');
  });

  it('labels the shield for its owner and for the master', () => {
    expect(shieldLabelForOwner(5)).toBe('Escudo Arcano +5 até a sua vez');
    expect(shieldLabelForMaster(5)).toBe('Escudo Arcano +5 até a vez dele');
  });

  it('says the shield ended, with the armor class that came back', () => {
    expect(shieldEndedSentence(13)).toBe('O Escudo Arcano acabou. A sua CA voltou a 13.');
    expect(shieldEndedParts(13)).toEqual({
      lead: 'O Escudo Arcano acabou.',
      rest: 'A sua CA voltou a 13.',
    });
    expect(shieldEndedSentence(null)).toBe('O Escudo Arcano acabou.');
    expect(shieldEndedForMaster('Pensantus')).toBe('O Escudo Arcano de Pensantus acabou');
  });

  it('labels Ajuda with the bonus the server sent: +5, and +10 for a third-level slot', () => {
    expect(aidTag(5)).toBe('+5 de Ajuda');
    expect(aidTag(10)).toBe('+10 de Ajuda');
    expect(aidTagForMaster(5)).toBe('+5 Ajuda');
    expect(aidRowLabel(5)).toBe('Ajuda +5 PV');
    expect(aidLabel(10)).toBe('Ajuda: +10 nos PV até o mestre encerrar ou um descanso longo');
  });

  it('reads the hit points for a screen reader, with Ajuda only under it', () => {
    expect(vitalsSpeech(43, 43, 5)).toBe('Pontos de vida: 43 de 43, 5 de Ajuda');
    expect(vitalsSpeech(30, 30, 0)).toBe('Pontos de vida: 30 de 30');
  });

  it("splits the bar into the sheet's own part and the striped part of Ajuda", () => {
    expect(aidBar(43, 43, 5).solid).toBeCloseTo((38 / 43) * 100);
    expect(aidBar(43, 43, 5).striped).toBeCloseTo((5 / 43) * 100);
    expect(aidBar(31, 43, 5)).toEqual({ solid: (31 / 43) * 100, striped: 0 });
    expect(aidBar(40, 43, 5).striped).toBeCloseTo((2 / 43) * 100);
    expect(aidBar(0, 0, 0)).toEqual({ solid: 0, striped: 0 });
  });

  it('writes the question of ending Ajuda: the cut when it is above the new maximum, the stay when it is not', () => {
    expect(sheetMaximum(43, 5)).toBe(38);
    expect(endAidText(43, 43, 5)).toBe(
      'O máximo de PV volta a 38. Os PV atuais passam de 43 para 38: o que passa do novo máximo se perde, e isto não se desfaz.',
    );
    expect(endAidText(31, 43, 5)).toBe(
      'O máximo de PV volta a 38. Os PV atuais ficam em 31: só o máximo cai, e isto não se desfaz.',
    );
    expect(endAidText(40, 45, 5)).toContain('Os PV atuais ficam em 40');
  });
});
