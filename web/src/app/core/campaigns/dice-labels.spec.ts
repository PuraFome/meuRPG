import { DiceMode, DicePreference } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { effectivePreference, preferenceLabel } from './dice-labels';

describe('dice labels', () => {
  it('names the two preferences as the design does', () => {
    expect(preferenceLabel(DicePreference.APP)).toBe('No app');
    expect(preferenceLabel(DicePreference.PHYSICAL)).toBe('Meus próprios dados');
  });

  it('lets the preference count only while players choose', () => {
    expect(effectivePreference(DiceMode.PLAYERS_CHOOSE, DicePreference.PHYSICAL)).toBe(DicePreference.PHYSICAL);
    expect(effectivePreference(DiceMode.PLAYERS_CHOOSE, DicePreference.UNSPECIFIED)).toBe(DicePreference.APP);
    expect(effectivePreference(DiceMode.APP, DicePreference.PHYSICAL)).toBe(DicePreference.APP);
    expect(effectivePreference(DiceMode.PHYSICAL, DicePreference.APP)).toBe(DicePreference.PHYSICAL);
  });
});
