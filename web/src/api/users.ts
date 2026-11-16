import { http } from "@/utils/http";

export interface UserTeam {
  team_id: number;
  name: string;
}

export interface UserProject {
  project_id: number;
  name: string;
}

export interface UserRole {
  id: number;
  code: string;
  name: string;
}

export interface RoutePermission {
  id: number;
  code: string;
  name: string;
  path: string;
}

export interface SystemUser {
  id: number;
  username: string;
  is_admin: boolean;
  disabled: boolean;
  teams: UserTeam[];
  projects: UserProject[];
  roles: UserRole[];
  permissions: RoutePermission[];
}

export interface CreateUserPayload {
  username: string;
  password: string;
  is_admin: boolean;
  project_ids: number[];
  role_ids: number[];
}

export interface UpdateUserPayload {
  username: string;
  password?: string;
  is_admin: boolean;
  disabled: boolean;
  project_ids: number[];
  role_ids: number[];
}

export interface Role {
  id: number;
  code: string;
  name: string;
  permission_ids: number[];
  permissions: RoutePermission[];
}

export interface RolePayload {
  code: string;
  name: string;
  permission_ids: number[];
}

export interface PermissionPayload {
  code: string;
  name: string;
  path: string;
}

export const listUsers = () => http.request<SystemUser[]>("get", "/api/users");

export const createUser = (data: CreateUserPayload) =>
  http.request<{ id: number; username: string }>("post", "/api/users", {
    data
  });

export const updateUser = (id: number, data: UpdateUserPayload) =>
  http.request<{ updated: boolean }>("put", `/api/users/${id}`, { data });

export const deleteUser = (id: number) =>
  http.request<{ deleted: boolean }>("delete", `/api/users/${id}`);

export const listRoles = () => http.request<Role[]>("get", "/api/roles");

export const createRole = (data: RolePayload) =>
  http.request<Role>("post", "/api/roles", { data });

export const updateRole = (id: number, data: RolePayload) =>
  http.request<{ updated: boolean }>("put", `/api/roles/${id}`, { data });

export const deleteRole = (id: number) =>
  http.request<{ deleted: boolean }>("delete", `/api/roles/${id}`);

export const listPermissions = () =>
  http.request<RoutePermission[]>("get", "/api/permissions");

export const createPermission = (data: PermissionPayload) =>
  http.request<RoutePermission>("post", "/api/permissions", { data });

export const updatePermission = (id: number, data: PermissionPayload) =>
  http.request<{ updated: boolean }>("put", `/api/permissions/${id}`, { data });

export const deletePermission = (id: number) =>
  http.request<{ deleted: boolean }>("delete", `/api/permissions/${id}`);
