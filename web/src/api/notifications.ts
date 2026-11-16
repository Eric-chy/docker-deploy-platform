import { http } from "@/utils/http";

export interface NotificationChannel {
  id: number;
  user_id: number;
  username: string;
  role_ids: number[];
  role_names: string[];
  type: string;
  name: string;
  webhook_url: string;
  secret_configured: boolean;
  enabled: boolean;
  team_ids: number[];
  created_at: string;
  updated_at: string;
}

export interface NotificationChannelPayload {
  role_ids: number[];
  type: string;
  name: string;
  webhook_url: string;
  secret?: string;
  enabled: boolean;
  team_ids: number[];
}

export const listNotificationChannels = () =>
  http.request<NotificationChannel[]>("get", "/api/notification-channels");

export const createNotificationChannel = (data: NotificationChannelPayload) =>
  http.request<{ id: number }>("post", "/api/notification-channels", { data });

export const updateNotificationChannel = (
  id: number,
  data: NotificationChannelPayload
) =>
  http.request<{ updated: boolean }>(
    "put",
    `/api/notification-channels/${id}`,
    { data }
  );

export const deleteNotificationChannel = (id: number) =>
  http.request<{ deleted: boolean }>(
    "delete",
    `/api/notification-channels/${id}`
  );
