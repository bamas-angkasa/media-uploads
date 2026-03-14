export interface User {
  id: string;
  email: string;
  username: string;
  role: "user" | "admin";
  plan: "free" | "premium";
  storage_quota: number;
  storage_used: number;
  created_at: string;
}

export interface Media {
  id: string;
  user_id: string;
  short_code: string;
  type: "image" | "video" | "gif";
  title?: string;
  description?: string;
  tags?: string[];
  status: "processing" | "ready" | "failed";
  file_size: number;
  mime_type: string;
  width?: number;
  height?: number;
  duration_sec?: number;
  view_count: number;
  download_count: number;
  thumbnail_url?: string;
  created_at: string;
  updated_at: string;
}

export interface MediaFile {
  id: string;
  media_id: string;
  variant: "original" | "thumb" | "1080p" | "hls_master" | "poster";
  s3_key: string;
  width?: number;
  height?: number;
  file_size?: number;
  format?: string;
}

export interface Report {
  id: string;
  media_id: string;
  reporter_id: string;
  reason: string;
  status: "pending" | "resolved" | "dismissed";
  created_at: string;
  media_short_code?: string;
  media_title?: string;
  reporter?: string;
}

export interface PaginatedResponse<T> {
  data: T[];
  total?: number;
  page?: number;
  page_size?: number;
  next_cursor?: string;
}

export interface ProcessingStatus {
  media_id: string;
  status: "processing" | "ready" | "failed";
  progress: number;
  error?: string;
}

export interface SignResponse {
  upload_url: string;
  media_id: string;
  short_code: string;
  expires_at: number;
}

export interface PlatformStats {
  total_media: number;
  total_users: number;
  total_storage_gb: number;
  media_last_24h: number;
  users_last_7d: number;
  pending_reports: number;
}
