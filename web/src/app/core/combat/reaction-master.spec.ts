import { CombatantKind, ReactionKind } from '../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter, reactionWindow } from './combat-testing';
import {
  describeRow,
  heldReason,
  reactionCards,
  reactionName,
  slotOptions,
  windowsBarText,
} from './reaction-master';

const plain = (text: string) => text.replace(/ /g, ' ');

const combatants = [
  combatant({ id: 't', label: 'Toren', kind: CombatantKind.PLAYER }),
  combatant({ id: 'pen', label: 'Pensantus', kind: CombatantKind.PLAYER }),
  combatant({ id: 'nael', label: 'Nael', kind: CombatantKind.PLAYER }),
  combatant({ id: 'm1', label: 'Mago 1' }),
  combatant({ id: 'hob', label: 'Hobgoblin' }),
  combatant({ id: 'brisa', label: 'Brisa', kind: CombatantKind.PLAYER }),
];

const spell = {
  actorId: 'm1',
  actorLabel: 'Mago 1',
  actionNamePt: 'Bola de Fogo',
  distanceFt: 30,
  slotEffects: [],
};

function counter(id: string, reactor: string, label: string, over: object = {}) {
  return reactionWindow({
    id,
    kind: ReactionKind.COUNTERSPELL,
    groupId: 'spell-1',
    reactorId: reactor,
    reactorLabel: label,
    reactorIsPlayer: true,
    trigger: spell as never,
    ...over,
  } as never);
}

describe("the master's cards for the reaction windows", () => {
  it('groups the windows of one action in a queue, in the order given, the first to answer and the others after it', () => {
    const e = encounter({
      combatants,
      reactionWindows: [
        counter('a', 'pen', 'Pensantus', { answerNow: true }),
        counter('b', 'nael', 'Nael', { answerNow: false }),
      ],
    } as never);
    const cards = reactionCards(e);
    expect(cards).toHaveLength(1);
    const card = cards[0];
    expect(card.type).toBe('queue');
    if (card.type !== 'queue') {
      return;
    }
    expect(card.title).toBe('Reações a uma magia');
    expect(plain(card.subtitle)).toBe('Bola de Fogo do Mago 1 · na ordem da iniciativa');
    expect(card.rows.map((r) => r.label)).toEqual(['Pensantus', 'Nael']);
    expect(card.rows.map((r) => r.tag)).toEqual(['Jogador', 'Jogador']);
    expect(plain(card.rows[0].status)).toBe('Respondendo no celular · vez de responder');
    expect(plain(card.rows[1].status)).toBe('Depois de Pensantus');
    expect(card.rows.map((r) => r.answerNow)).toEqual([true, false]);
  });

  it('never guesses a pronoun for a player: "pelo jogador"', () => {
    const e = encounter({
      combatants,
      reactionWindows: [
        counter('a', 'pen', 'Pensantus'),
        counter('b', 'brisa', 'Brisa', { answerNow: false }),
      ],
    } as never);
    const card = reactionCards(e)[0];
    if (card.type !== 'queue') {
      throw new Error('a queue');
    }
    for (const row of card.rows) {
      expect(row.useLabel).toBe('Usar pelo jogador');
      expect(row.passLabel).toBe('Deixar passar pelo jogador');
    }
    const text = JSON.stringify(
      card.rows.map((r) => [r.useLabel, r.passLabel, r.description, r.status]),
    );
    expect(text).not.toMatch(/por ele|por ela| ele | ela /);
  });

  it('two actions are two cards: a spell and an attack', () => {
    const e = encounter({
      combatants,
      reactionWindows: [
        counter('a', 'pen', 'Pensantus'),
        reactionWindow({
          id: 'u',
          kind: ReactionKind.UNCANNY_DODGE,
          groupId: 'attack-1',
          reactorId: 'brisa',
          reactorLabel: 'Brisa',
          reactorIsPlayer: true,
          trigger: {
            actorId: 'hob',
            actorLabel: 'Hobgoblin',
            targetLabel: 'Brisa',
            slotEffects: [],
          } as never,
        }),
      ],
    } as never);
    const cards = reactionCards(e);
    expect(cards.map((c) => (c.type === 'queue' ? c.title : c.type))).toEqual([
      'Reações a uma magia',
      'Reações a um ataque',
    ]);
    const attack = cards[1];
    if (attack.type === 'queue') {
      expect(plain(attack.subtitle)).toBe('Ataque do Hobgoblin contra Brisa · acertou');
      expect(plain(attack.rows[0].description)).toBe(
        'Esquiva Sobrenatural · o ataque atingiu Brisa · vê o Hobgoblin',
      );
    }
  });

  it("an NPC alone is a card of its own, with Escudo and the numbers that are the master's", () => {
    const e = encounter({
      combatants,
      reactionWindows: [
        reactionWindow({
          id: 's',
          kind: ReactionKind.SHIELD,
          reactorId: 'm1',
          reactorLabel: 'Mago 1',
          trigger: {
            actorId: 't',
            actorLabel: 'Toren',
            attackTotal: 15,
            armorClassWithShield: 17,
            slotEffects: [],
          } as never,
        }),
      ],
    } as never);
    const card = reactionCards(e)[0];
    expect(card.type).toBe('single');
    if (card.type === 'single') {
      expect(plain(card.text)).toBe(
        'Esperando a sua reação: o Mago 1 pode conjurar Escudo Arcano (+5 na CA: 17 contra 15 viraria erro). O jogador de Toren lê só “Esperando o mestre”.',
      );
      expect(card.row.useLabel).toBe('Usar Escudo Arcano pelo Mago 1');
      expect(card.row.passLabel).toBe('Deixar passar');
      expect(card.row.tag).toBe('NPC');
    }
  });

  it("an NPC's Counterspell lists the slots with what each does", () => {
    const w = counter('c', 'm1', 'Mago 1', {
      reactorIsPlayer: false,
      trigger: {
        ...spell,
        actorId: 'pen',
        actorLabel: 'Pensantus',
        actionNamePt: 'Bola de Fogo',
        spellKey: 'spell:fireball',
        spellLevel: 3,
        slotEffects: [
          { slot: { level: 3, pact: false, free: 2 }, noCheck: true, checkDc: 0, checkBonus: 0 },
          { slot: { level: 4, pact: false, free: 1 }, noCheck: false, checkDc: 13, checkBonus: 3 },
        ],
      },
    });
    expect(slotOptions(w).map((o) => [plain(o.title), o.effect])).toEqual([
      ['3º nível', 'anula sem teste'],
      ['4º nível', 'teste de conjuração CD 13'],
    ]);
    const card = reactionCards(encounter({ combatants, reactionWindows: [w] } as never))[0];
    expect(card.type).toBe('single');
    if (card.type === 'single') {
      expect(plain(card.text)).toContain('pode usar Contramágica contra Bola de Fogo (3º nível)');
    }
  });

  it('the second steps and the check are their own cards', () => {
    const e = encounter({
      combatants,
      reactionWindows: [
        reactionWindow({
          id: 'check',
          kind: ReactionKind.MASTER_CHECK,
          prompt: {
            case: 'masterCheck',
            value: {
              summaryPt: 'Ataque de Toren contra o Goblin 2 · acertou',
              enemyCanReact: false,
            },
          } as never,
        }),
        reactionWindow({
          id: 'save',
          kind: ReactionKind.HELLISH_REBUKE,
          secondStep: true,
          reactorLabel: 'Mirta',
          prompt: {
            case: 'hellishRebukeSave',
            value: {
              aggressorLabel: 'Hobgoblin',
              saveDc: 13,
              saveBonus: 1,
              bonusKnown: true,
              diceCount: 3,
            },
          } as never,
        }),
        reactionWindow({
          id: 'conc',
          kind: ReactionKind.CONCENTRATION_SAVE,
          reactorId: 'm1',
          reactorLabel: 'Mago 2',
        }),
        reactionWindow({
          id: 'conc2',
          kind: ReactionKind.CONCENTRATION_SAVE,
          reactorId: 'brisa',
          reactorLabel: 'Sálvia',
          reactorIsPlayer: true,
        }),
      ],
    } as never);
    const cards = reactionCards(e);
    expect(cards.map((c) => c.type)).toEqual([
      'check',
      'rebukeSave',
      'concentration',
      'concentration',
    ]);
    expect(
      cards
        .filter((c) => c.type === 'concentration')
        .map((c) => c.type === 'concentration' && c.player),
    ).toEqual([false, true]);
  });

  it("skips the windows that are not the master's to show: opportunity and the answered ones", () => {
    const e = encounter({
      combatants,
      reactionWindows: [reactionWindow({ id: 'o', kind: ReactionKind.OPPORTUNITY })],
    } as never);
    expect(reactionCards(e)).toEqual([]);
  });
});

describe("what the master's held actions say", () => {
  it('names the reactor, or the request above, or the save', () => {
    expect(heldReason(encounter())).toBe('');
    const shield = encounter({
      reactionWindows: [reactionWindow({ id: 's', reactorLabel: 'Mago 1' })],
    } as never);
    expect(plain(heldReason(shield))).toBe('Espere a reação do Mago 1.');
    const check = encounter({
      reactionWindows: [reactionWindow({ id: 'c', kind: ReactionKind.MASTER_CHECK })],
    } as never);
    expect(heldReason(check)).toBe('Responda ao pedido acima.');
    const save = encounter({
      reactionWindows: [
        reactionWindow({
          id: 'k',
          kind: ReactionKind.CONCENTRATION_SAVE,
          reactorLabel: 'Sálvia',
          reactorIsPlayer: true,
        }),
      ],
    } as never);
    expect(heldReason(save)).toBe('Espere o teste de concentração de Sálvia.');
  });

  it("writes the master's bar: his own reaction, or the player he waits on", () => {
    const npc = encounter({
      combatants,
      reactionWindows: [reactionWindow({ id: 's', reactorId: 'm1', reactorLabel: 'Mago 1' })],
    } as never);
    expect(windowsBarText(npc)).toBe('Esperando a sua reação: Mago 1');
    const player = encounter({
      combatants,
      reactionWindows: [counter('a', 'pen', 'Pensantus')],
    } as never);
    expect(windowsBarText(player)).toBe('Esperando a reação de Pensantus');
    expect(windowsBarText(encounter())).toBe('');
  });

  it('names each reaction as names_pt.json does', () => {
    expect(reactionName(reactionWindow({ id: 'x', kind: ReactionKind.CUTTING_WORDS }))).toBe(
      'Palavras de Interrupção',
    );
    expect(reactionName(reactionWindow({ id: 'x', kind: ReactionKind.FEATHER_FALL }))).toBe(
      'Queda Suave',
    );
    expect(describeRow(reactionWindow({ id: 'x', kind: ReactionKind.DEFLECT_MISSILES }))).toContain(
      'Defletir Projéteis',
    );
  });
});
