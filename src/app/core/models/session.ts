export interface QuickReference {
  id: string;
  title: string;
  content: string;
}

export interface ToolbarShortcut {
  id: string;
  icon: string;
  label: string;
  action: string;
}

export interface SessionState {
  id: string;
  campaignId?: string;
  activeEntity?: {
    type: 'character' | 'map' | 'rules' | 'note';
    id: string;
  };
  quickReferences: QuickReference[];
  toolbarShortcuts: ToolbarShortcut[];
  notes: string;
  createdAt: Date;
  updatedAt: Date;
}
