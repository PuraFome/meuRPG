import type { AttributeKey } from '../../core/models/character';

export interface AbilityDefinition {
  key: AttributeKey;
  /** Full Portuguese label, e.g. "Força". */
  label: string;
  /** Short label used in dense grids, e.g. "FOR". */
  short: string;
}

export const ABILITIES: readonly AbilityDefinition[] = [
  { key: 'for', label: 'Força', short: 'FOR' },
  { key: 'des', label: 'Destreza', short: 'DES' },
  { key: 'con', label: 'Constituição', short: 'CON' },
  { key: 'int', label: 'Inteligência', short: 'INT' },
  { key: 'sab', label: 'Sabedoria', short: 'SAB' },
  { key: 'car', label: 'Carisma', short: 'CAR' },
];

export const ATTRIBUTE_KEYS: readonly AttributeKey[] = ABILITIES.map((a) => a.key);

const ABILITY_LABELS: Record<AttributeKey, string> = {
  for: 'Força',
  des: 'Destreza',
  con: 'Constituição',
  int: 'Inteligência',
  sab: 'Sabedoria',
  car: 'Carisma',
};

export function abilityLabel(key: AttributeKey): string {
  return ABILITY_LABELS[key];
}

export interface SkillDefinition {
  name: string;
  ability: AttributeKey;
}

/** The 18 canonical 5e skills, mapped to their governing ability. */
export const DND_SKILLS: readonly SkillDefinition[] = [
  { name: 'Acrobacia', ability: 'des' },
  { name: 'Adestrar Animais', ability: 'sab' },
  { name: 'Arcanismo', ability: 'int' },
  { name: 'Atletismo', ability: 'for' },
  { name: 'Atuação', ability: 'car' },
  { name: 'Enganação', ability: 'car' },
  { name: 'Furtividade', ability: 'des' },
  { name: 'História', ability: 'int' },
  { name: 'Intimidação', ability: 'car' },
  { name: 'Intuição', ability: 'sab' },
  { name: 'Investigação', ability: 'int' },
  { name: 'Medicina', ability: 'sab' },
  { name: 'Natureza', ability: 'int' },
  { name: 'Percepção', ability: 'sab' },
  { name: 'Persuasão', ability: 'car' },
  { name: 'Prestidigitação', ability: 'des' },
  { name: 'Religião', ability: 'int' },
  { name: 'Sobrevivência', ability: 'sab' },
];

const SKILL_ABILITY = new Map<string, AttributeKey>(
  DND_SKILLS.map((s) => [s.name, s.ability]),
);

/** Governing ability for a skill name, defaulting to FOR for homebrew skills. */
export function skillAbility(name: string): AttributeKey {
  return SKILL_ABILITY.get(name) ?? 'for';
}

export type SpellcasterKind = 'full' | 'half' | 'third' | 'pact' | 'none';

export interface ClassDefinition {
  name: string;
  hitDice: string;
  savingThrows: AttributeKey[];
  spellcasting: SpellcasterKind;
  spellcastingAbility: AttributeKey | null;
}

/** Core 5e classes with their hit die, saving-throw proficiencies and spellcasting. */
export const DND_CLASSES: readonly ClassDefinition[] = [
  { name: 'Bárbaro', hitDice: '1d12', savingThrows: ['for', 'con'], spellcasting: 'none', spellcastingAbility: null },
  { name: 'Bardo', hitDice: '1d8', savingThrows: ['des', 'car'], spellcasting: 'full', spellcastingAbility: 'car' },
  { name: 'Bruxo', hitDice: '1d8', savingThrows: ['sab', 'car'], spellcasting: 'pact', spellcastingAbility: 'car' },
  { name: 'Clérigo', hitDice: '1d8', savingThrows: ['sab', 'car'], spellcasting: 'full', spellcastingAbility: 'sab' },
  { name: 'Druida', hitDice: '1d8', savingThrows: ['int', 'sab'], spellcasting: 'full', spellcastingAbility: 'sab' },
  { name: 'Feiticeiro', hitDice: '1d6', savingThrows: ['con', 'car'], spellcasting: 'full', spellcastingAbility: 'car' },
  { name: 'Guerreiro', hitDice: '1d10', savingThrows: ['for', 'con'], spellcasting: 'none', spellcastingAbility: null },
  { name: 'Ladino', hitDice: '1d8', savingThrows: ['des', 'int'], spellcasting: 'none', spellcastingAbility: null },
  { name: 'Mago', hitDice: '1d6', savingThrows: ['int', 'sab'], spellcasting: 'full', spellcastingAbility: 'int' },
  { name: 'Monge', hitDice: '1d8', savingThrows: ['for', 'des'], spellcasting: 'none', spellcastingAbility: null },
  { name: 'Paladino', hitDice: '1d10', savingThrows: ['sab', 'car'], spellcasting: 'half', spellcastingAbility: 'car' },
  { name: 'Patrulheiro', hitDice: '1d10', savingThrows: ['for', 'des'], spellcasting: 'half', spellcastingAbility: 'sab' },
];

const CLASS_BY_NAME = new Map<string, ClassDefinition>(DND_CLASSES.map((c) => [c.name, c]));
const CLASS_BY_NAME_LOWER = new Map<string, ClassDefinition>(
  DND_CLASSES.map((c) => [c.name.toLowerCase(), c]),
);

/** Case-insensitive class lookup; returns undefined for homebrew class names. */
export function findClass(name: string): ClassDefinition | undefined {
  return CLASS_BY_NAME.get(name) ?? CLASS_BY_NAME_LOWER.get(name.trim().toLowerCase());
}

export interface RaceDefinition {
  name: string;
  /** Walking speed in feet. */
  speed: number;
}

export const DND_RACES: readonly RaceDefinition[] = [
  { name: 'Anão', speed: 25 },
  { name: 'Elfo', speed: 30 },
  { name: 'Halfling', speed: 25 },
  { name: 'Humano', speed: 30 },
  { name: 'Draconato', speed: 30 },
  { name: 'Gnomo', speed: 25 },
  { name: 'Meio-Elfo', speed: 30 },
  { name: 'Meio-Orc', speed: 30 },
  { name: 'Tiefling', speed: 30 },
];

const RACE_SPEED = new Map<string, number>(DND_RACES.map((r) => [r.name, r.speed]));

/** Walking speed for a race name, defaulting to 30 ft for homebrew races. */
export function raceSpeed(name: string): number {
  return RACE_SPEED.get(name) ?? 30;
}

export const DND_BACKGROUNDS: readonly string[] = [
  'Acólito',
  'Artesão de Guilda',
  'Charlatão',
  'Criminoso',
  'Eremita',
  'Forasteiro',
  'Herói do Povo',
  'Marinheiro',
  'Nobre',
  'Órfão',
  'Sábio',
  'Soldado',
];

export const DND_ALIGNMENTS: readonly string[] = [
  'Leal e Bom',
  'Neutro e Bom',
  'Caótico e Bom',
  'Leal e Neutro',
  'Neutro',
  'Caótico e Neutro',
  'Leal e Mau',
  'Neutro e Mau',
  'Caótico e Mau',
];

export const DND_HIT_DICE: readonly string[] = ['1d4', '1d6', '1d8', '1d10', '1d12'];

export const DND_LANGUAGES: readonly string[] = [
  'Comum',
  'Anão',
  'Élfico',
  'Gigante',
  'Gnômico',
  'Goblin',
  'Halfling',
  'Orc',
  'Abissal',
  'Celestial',
  'Dracônico',
  'Infernal',
  'Silvestre',
  'Subcomum',
];

export const DND_ARMOR_PROFICIENCIES: readonly string[] = [
  'Armaduras leves',
  'Armaduras médias',
  'Armaduras pesadas',
  'Escudos',
];

export const DND_WEAPON_PROFICIENCIES: readonly string[] = [
  'Armas simples',
  'Armas marciais',
];

export const DND_TOOLS: readonly string[] = [
  'Ferramentas de ladrão',
  'Ferramentas de artesão',
  'Instrumento musical',
  'Kit de herbalismo',
  'Kit de disfarce',
  'Kit de falsificação',
  'Kit de veneno',
  'Jogo de dados',
];

export const DND_CONDITIONS: readonly string[] = [
  'Amedrontado',
  'Atordoado',
  'Cego',
  'Enfeitiçado',
  'Envenenado',
  'Exausto',
  'Imobilizado',
  'Incapacitado',
  'Inconsciente',
  'Invisível',
  'Paralisado',
  'Petrificado',
  'Prono',
  'Restringido',
  'Surdo',
];