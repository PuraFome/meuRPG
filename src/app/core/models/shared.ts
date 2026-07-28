import type { CampaignFolder } from './campaign';
import type { Character } from './character';
import type { GalleryItem } from './gallery';
import type { MapData } from './map';
import type { RuleBook } from './rules';
import type { SessionState } from './session';

export type EntityType =
  | 'character'
  | 'campaign'
  | 'gallery'
  | 'map'
  | 'session'
  | 'rules';

export interface SearchResult {
  id: string;
  type: EntityType;
  name: string;
  description: string;
  score: number;
  entity:
    | Character
    | CampaignFolder
    | GalleryItem
    | MapData
    | SessionState
    | RuleBook;
}

export interface SidebarItem {
  id: string;
  label: string;
  icon: string;
  route: string;
  badge?: number;
  children?: SidebarItem[];
}
