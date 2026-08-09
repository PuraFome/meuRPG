export interface CampaignFolder {
  id: string;
  name: string;
  parentId: string | null;
  children: CampaignFolder[];
  entityIds: {
    characterIds: string[];
    mapIds: string[];
    sessionIds: string[];
  };
  /**
   * IndexedDB blob id of the full-size cover. Thumbnail lives at `${coverFileId}-thumb`.
   */
  coverFileId?: string | null;
  /**
   * URL of the campaign guide document (Google Docs link). Shown as an item in the folder.
   */
  googleDocUrl?: string;
  /**
   * Legacy rich-text HTML from the previous in-app guide editor. Kept only to
   * migrate the link the user pasted there into `googleDocUrl` on first load.
   */
  guideHtml?: string;
}
