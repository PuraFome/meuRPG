import { create } from '@bufbuild/protobuf';

import { TableContentViolationSchema } from '../../../gen/meurpg/rules/v1/table_content_pb';
import { placeViolations, violationText } from './content-violations';
import { HIT_TRIGGER_PT } from './content-violation-texts-hit';
import { emptyHit, hitFieldPaths } from './hit-effect-draft';

const v = (field: string, reason: string) =>
  create(TableContentViolationSchema, { field, reason, key: '' });
const CTX = { aOne: 'uma magia' };
const AT = 'table_spell.effect.modifiers[0]';

describe('the refusals of "Efeito no próximo golpe" in words (#577)', () => {
  it('says the effect has at most 8 modifiers', () => {
    expect(violationText(v('table_spell.effect.modifiers', 'limit'), CTX)).toBe(
      'Um efeito tem no máximo 8 modificadores. Tire algum antes de ligar este.',
    );
  });

  it.each(['on_fail_conditions[0]', 'on_fail_conditions[3]'])(
    'tells a condition the server does not know, or lists twice, at %s',
    (leaf) => {
      expect(violationText(v(`${AT}.${leaf}`, 'bad_value'), CTX)).toBe(
        'Esta condição não existe ou está repetida. Escolha cada condição uma vez.',
      );
    },
  );

  it('keeps the pair of the test and the condition at the list itself', () => {
    expect(violationText(v(`${AT}.on_fail_conditions`, 'bad_value'), CTX)).toBe(
      'Escolha a condição de quem falhar, ou tire o teste.',
    );
  });

  it('says the trigger is to be chosen', () => {
    expect(HIT_TRIGGER_PT).toBe('Escolha quando vale.');
    expect(violationText(v(`${AT}.trigger`, 'bad_value'), CTX)).toContain(HIT_TRIGGER_PT);
  });

  it('puts an indexed refusal of the conditions on the condition input', () => {
    const known = new Set(hitFieldPaths(undefined).map((p) => `table_spell.${p}`));
    const placed = placeViolations(
      [v(`${AT}.on_fail_conditions[1]`, 'bad_value')],
      (p) => known.has(p),
      CTX,
    );
    expect(placed.fields).toEqual([`${AT}.on_fail_conditions`]);
    expect(placed.top).toEqual([]);
    expect(emptyHit().enabled).toBe(false);
  });
});
