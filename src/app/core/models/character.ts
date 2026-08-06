export interface DndSheet {
  // Identidade
  race: string;
  class: string;
  level: number;
  background: string;
  alignment: string;
  xp: number;
  // Combate
  hpMax: number;
  hpCurrent: number;
  hpTemp: number;
  armorClass: number;
  initiative: number;
  speed: number;
  hitDice: string;
  proficiencyBonus: number;
  // Proficiências e idiomas
  proficiencies: string[];
  languages: string[];
  // Traços e características
  features: string[];
}

export interface Character {
  id: string;
  name: string;
  description: string;
  type: 'npc' | 'player' | 'boss';
  imageUrl?: string;
  history?: string;
  masterNotes?: string;
  attributes: Record<string, number>;
  skills: string[];
  inventory: string[];
  quotes: string[];
  /** Full D&D sheet for player/boss types; absent for NPC. */
  sheet?: DndSheet;
  createdAt: Date;
  updatedAt: Date;
}
