import { FeatureEconomy } from '../../../gen/meurpg/play/v1/combat_pb';
import { featureCostText } from './combat-log-feature-cost';

describe('featureCostText', () => {
  const base = {
    economy: FeatureEconomy.REACTION,
    resourceNamePt: '',
    resourceLeft: 0,
    resourceMax: 0,
  };

  it('is empty when the entry says nothing', () => {
    expect(featureCostText({ ...base, economy: FeatureEconomy.UNSPECIFIED })).toBe('');
  });

  it('says what is left of how many', () => {
    expect(
      featureCostText({
        economy: FeatureEconomy.NONE,
        resourceNamePt: 'Forma Selvagem',
        resourceLeft: 2,
        resourceMax: 3,
      }),
    ).toBe('gastou 1 uso de Forma Selvagem (restam 2 de 3)');
  });

  it('leaves "de M" out when the entry has no most', () => {
    expect(featureCostText({ ...base, resourceNamePt: 'Forma Selvagem', resourceLeft: 2 })).toBe(
      'gastou a reação e 1 uso de Forma Selvagem (restam 2)',
    );
  });
});
