import { CombatantKind, ReactionKind } from '../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter, reactionWindow } from './combat-testing';
import {
  LEGENDARY_UNUSABLE,
  legendaryCard,
  legendaryCards,
  legendaryFolded,
  legendaryNames,
  legendaryRecovered,
  waitingBehind,
} from './legendary-card';

const options = [
  {
    key: 'detect',
    name: 'Detect',
    namePt: 'Detectar',
    cost: 1,
    kind: 'text',
    text: 'English text',
    affordable: true,
    attackKey: '',
  },
  {
    key: 'tail',
    name: 'Tail Attack',
    namePt: 'Ataque de cauda',
    cost: 1,
    kind: 'attack',
    text: 'x',
    affordable: true,
    attackKey: 'm#tail',
  },
  {
    key: 'wing',
    name: 'Wing Attack',
    namePt: 'Ataque de asas',
    cost: 2,
    kind: 'move',
    text: 'y',
    affordable: false,
    attackKey: '',
  },
];

function dragon(over: object = {}) {
  return combatant({
    id: 'd',
    label: 'Dragão vermelho adulto',
    legendary: {
      actionsPerRound: 3,
      actionsLeft: 1,
      resistancePerDay: 3,
      resistanceLeft: 2,
      usableNow: true,
      options,
      ...over,
    },
  } as never);
}

const tor = combatant({ id: 't', label: 'Tor', kind: CombatantKind.PLAYER });

describe("the master's legendary actions card", () => {
  it('counts the actions, names the options in Portuguese only and says what each costs', () => {
    const e = encounter({ combatants: [dragon(), tor], currentCombatantId: 't' });
    const card = legendaryCard(e, e.combatants[0]);
    expect(card?.title).toBe('Dragão vermelho adulto');
    expect(card?.counts).toBe('1 de 3');
    expect(card?.pips).toEqual([true, false, false]);
    expect(card?.spent).toBe(2);
    expect(card?.badge).toBe('Opcional');
    expect(card?.hint).toBe('Use no fim do turno de Tor, uma opção de cada vez.');
    expect(card?.rows.map((r) => r.name)).toEqual([
      'Detectar',
      'Ataque de cauda',
      'Ataque de asas',
    ]);
    expect(card?.rows.map((r) => r.costText)).toEqual(['custa 1', 'custa 1', 'custa 2']);
    const everything = JSON.stringify(card);
    expect(everything).not.toContain('Tail Attack');
    expect(everything).not.toContain('English text');
  });

  it('gives each kind its button: Usar, Atacar (with the attack key) and Usar to open the move', () => {
    const e = encounter({ combatants: [dragon(), tor], currentCombatantId: 't' });
    const rows = legendaryCard(e, e.combatants[0])?.rows ?? [];
    expect(rows.map((r) => [r.buttonLabel, r.action])).toEqual([
      ['Usar', 'spend'],
      ['Atacar', 'attack'],
      ['Usar', 'move'],
    ]);
    expect(rows[1].attackKey).toBe('m#tail');
    expect(rows[0].detail).toBe('Você resolve: o app só conta o gasto.');
  });

  it('turns off the option that costs more than what is left, and says so', () => {
    const e = encounter({ combatants: [dragon(), tor], currentCombatantId: 't' });
    const rows = legendaryCard(e, e.combatants[0])?.rows ?? [];
    expect(rows[0].disabled).toBe(false);
    expect(rows[2].disabled).toBe(true);
    expect(rows[2].reason).toBe('Custa 2; restam 1.');
  });

  it('turns every row off and reads "Depois" when the creature cannot act', () => {
    const e = encounter({
      combatants: [dragon({ usableNow: false, actionsLeft: 3 }), tor],
      currentCombatantId: 't',
    });
    const card = legendaryCard(e, e.combatants[0]);
    expect(card?.badge).toBe('Depois');
    expect(card?.unusable).toBe(LEGENDARY_UNUSABLE);
    expect(card?.rows.every((r) => r.disabled && r.reason === LEGENDARY_UNUSABLE)).toBe(true);
  });

  it('waits behind the open windows: "Depois", the rows held and the reason said', () => {
    const win = (id: string) =>
      reactionWindow({ id, kind: ReactionKind.PARRY, reactorId: 'c', reactorLabel: 'Capitão' });
    const e = encounter({
      combatants: [dragon({ actionsLeft: 3 }), tor],
      currentCombatantId: 't',
      reactionWindows: [win('a'), win('b')],
    });
    const card = legendaryCard(e, e.combatants[0]);
    expect(card?.badge).toBe('Depois');
    expect(card?.blocked).toBe(true);
    expect(card?.waiting).toBe(
      'Depois das duas janelas acima: enquanto uma reação espera, nada anda.',
    );
    expect(waitingBehind(1)).toBe('Depois da janela acima: enquanto uma reação espera, nada anda.');
    expect(waitingBehind(0)).toBe('');
  });

  it('folds to "Ações lendárias · 3 de 3" as plain data', () => {
    const e = encounter({
      combatants: [dragon({ actionsLeft: 3 }), tor],
      currentCombatantId: 't',
    });
    const card = legendaryCard(e, e.combatants[0]);
    expect(card && legendaryFolded(card)).toBe('Ações lendárias · 3 de 3');
  });

  it('has no card for a creature without options or on its own turn, and says what came back instead', () => {
    const e = encounter({ combatants: [dragon(), tor], currentCombatantId: 'd' });
    expect(legendaryCards(e)).toEqual([]);
    const note = legendaryRecovered(e, e.combatants[0]);
    expect(note?.actions).toBe('Ações lendárias: 3 de 3, renovadas agora');
    expect(note?.resistance).toBe('Resistência Lendária: 2 de 3 hoje');
    expect(note?.note).toBe('Ações lendárias se usam no turno de outra criatura, nunca no dele.');
    expect(legendaryRecovered(e, tor)).toBeNull();
    const plain = encounter({ combatants: [tor], currentCombatantId: 'x' });
    expect(legendaryCards(plain)).toEqual([]);
  });

  it('leaves the resistance line out for a stat block without the trait', () => {
    const e = encounter({
      combatants: [dragon({ resistancePerDay: 0, resistanceLeft: 0 })],
      currentCombatantId: 'd',
    });
    expect(legendaryRecovered(e, e.combatants[0])?.resistance).toBe('');
  });

  it('maps the English option names of the log to the Portuguese ones', () => {
    const e = encounter({ combatants: [dragon()] });
    expect(legendaryNames(e).get('Wing Attack')).toBe('Ataque de asas');
  });
});
