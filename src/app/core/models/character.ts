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
  createdAt: Date;
  updatedAt: Date;
}
