import { FeatureTriggerKind } from '../../../gen/meurpg/rules/v1/rules_pb';
import { ReactionKind } from '../../../gen/meurpg/play/v1/combat_pb';
import { reactionWindow } from './combat-testing';
import { featureWindowView } from './reaction-feature';

const halo = (moment: FeatureTriggerKind, over: object = {}) =>
  reactionWindow({
    id: 'w',
    kind: ReactionKind.FEATURE,
    reactorId: 'sal',
    reactorLabel: 'Sálvia',
    reactorIsPlayer: true,
    trigger: {
      actorId: 'z1',
      actorLabel: 'Zumbi 1',
      actionKey: 'feature:halo',
      actionNamePt: 'Halo de esporos',
      distanceFt: 5,
      featureTrigger: moment,
      ...over,
    } as never,
  } as never);

const plain = (t: string) => t.replace(/\u00a0/g, ' ');

describe("the player's prompt of a feature window (board 743-02)", () => {
  it('names the feature, the creature and where it stands, for a creature that moved near', () => {
    const v = featureWindowView(halo(FeatureTriggerKind.ENEMY_MOVES_WITHIN), 2);
    expect(v.title).toBe('Halo de esporos');
    expect(v.subtitle).toBe('Reação · Zumbi 1 · Rodada 2');
    expect(plain(v.lead)).toBe('O Zumbi 1 chegou a 1,5 m de você.');
    expect(v.question).toBe('Usar o Halo de esporos?');
    expect(v.note).toBe(
      'Gasta a sua reação: ela volta no começo da sua próxima vez. O app registra o uso; o mestre resolve o resto.',
    );
    expect(v.useLabel('Zumbi 1')).toBe('Usar no Zumbi 1');
    expect(v.declineLabel).toBe('Não usar');
    expect(plain(v.ariaLabel)).toBe('O Zumbi 1 chegou a 1,5 m de você: usar o Halo de esporos?');
  });

  it('says the creature started its turn near, for the start of a turn', () => {
    const v = featureWindowView(halo(FeatureTriggerKind.ENEMY_STARTS_TURN_WITHIN), 2);
    expect(plain(v.lead)).toBe('O Zumbi 1 começou a vez a 1,5 m de você.');
  });

  it('agrees the articles with the names and leaves the distance out when the window has none', () => {
    const v = featureWindowView(
      halo(FeatureTriggerKind.ENEMY_MOVES_WITHIN, {
        actorLabel: 'Aranha',
        actionNamePt: 'Teia pegajosa',
        distanceFt: undefined,
      }),
      1,
    );
    expect(plain(v.lead)).toBe('A Aranha chegou perto de você.');
    expect(v.question).toBe('Usar a Teia pegajosa?');
    expect(v.useLabel('Aranha')).toBe('Usar na Aranha');
  });

  it('says "Reação" when the window has no feature name', () => {
    const v = featureWindowView(
      halo(FeatureTriggerKind.ENEMY_MOVES_WITHIN, { actionNamePt: '' }),
      1,
    );
    expect(v.title).toBe('Reação');
  });
});
