import { Code, ConnectError } from '@connectrpc/connect';

import { ExtraDiceState } from './extra-dice-state';

const refusal = (text: string) => new ConnectError(text, Code.InvalidArgument);

describe('ExtraDiceState', () => {
  it('asks for the dice a refusal says the roll takes', () => {
    const dice = new ExtraDiceState();
    expect(dice.fromRefusal(refusal('the roll takes 1 more die(s): type their faces'))).toBe(
      'Esta rolagem leva mais um d4: role-o e digite o resultado.',
    );
    expect(dice.fields().map((f) => f.label)).toEqual(['Resultado do d4 (Outra fonte)']);
    expect(dice.fromRefusal(refusal('the roll takes 2 more die(s): type'))).toContain('mais 2 d4');
    expect(dice.fields()).toHaveLength(2);
  });

  it('leaves any other refusal alone', () => {
    const dice = new ExtraDiceState();
    expect(dice.fromRefusal(refusal('the roll takes 2 d20'))).toBe('');
    expect(dice.fromRefusal(new ConnectError('the roll takes 1 more die(s)', Code.Aborted))).toBe(
      '',
    );
    expect(dice.fields()).toEqual([]);
  });

  it('sends nothing for the app dice or while no die was asked', () => {
    const dice = new ExtraDiceState();
    expect(dice.take(true)).toEqual([]);
    dice.fromRefusal(refusal('the roll takes 1 more die(s)'));
    expect(dice.take(false)).toEqual([]);
  });

  it('holds a typed roll back until every face is typed, then sends them in order', () => {
    const dice = new ExtraDiceState();
    dice.fromRefusal(refusal('the roll takes 2 more die(s)'));
    dice.faces.set([3, null]);
    expect(dice.take(true)).toBeNull();
    expect(dice.missingText()).toBe('Digite o resultado de cada dado extra antes de confirmar.');
    dice.faces.set([3, 1]);
    expect(dice.take(true)).toEqual([3, 1]);
  });
});
