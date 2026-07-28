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
}
