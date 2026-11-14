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

  it('says a use limited by its scope alone, with no resource name', () => {
    expect(featureCostText({ ...base, resourceLeft: 1, resourceMax: 2 })).toBe(
      'gastou a reação e 1 uso (restam 1 de 2)',
    );
  });

  it('has no parenthesis when both counts are 0', () => {
    expect(featureCostText(base)).toBe('gastou a reação');
    expect(featureCostText({ ...base, resourceNamePt: 'Forma Selvagem' })).toBe(
      'gastou a reação e 1 uso de Forma Selvagem',
    );
  });
});
