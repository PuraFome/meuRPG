export interface GalleryItem {
  id: string;
  name: string;
  type: 'image' | 'audio' | 'other';
  url: string;
  thumbnailUrl?: string;
  folderId: string | null;
  size: number;
  createdAt: Date;
}
