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
  /** Nome amigável da sessão (ex: "Sessão — Taverna do Dragão"). */
  name?: string;
  campaignId?: string;
  /** Mapa atualmente projetado na tela de apresentação. */
  activeMapId?: string | null;
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
