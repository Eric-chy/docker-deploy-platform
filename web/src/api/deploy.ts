import { http } from "@/utils/http";

export interface Project {
  id: number;
  name: string;
  team_id: number;
  current_version: string;
  latest_deployed_at: string;
  created_at: string;
}

export interface Team {
  id: number;
  name: string;
  created_at: string;
  role?: string;
}

export interface Node {
  id: number;
  name: string;
  address: string;
  status: string;
}

export interface ProjectDetail extends Project {
  current_yaml: string;
  node_id: number;
}

export interface Deployment {
  id: number;
  task_id: string;
  project?: string;
  version: string;
  action: string;
  status: string;
  created_at: string;
  duration_ms: number;
  operator_id: number;
  operator?: string;
  approver?: string;
  approval_status: string;
  approval_comment: string;
  approval_at?: string;
}

export interface DeploymentPage {
  items: Deployment[];
  total: number;
  page: number;
  page_size: number;
}

export interface DeploymentDetail extends Deployment {
  output: string;
}

export interface CreateProjectPayload {
  name: string;
  team_id: number;
  node_id: number;
  current_yaml: string;
  current_version: string;
}

export interface UpdateProjectPayload {
  name: string;
  team_id: number;
  node_id: number;
  current_yaml: string;
}

export const listProjects = () =>
  http.request<Project[]>("get", "/api/projects");

export const getProject = (id: number) =>
  http.request<ProjectDetail>("get", `/api/projects/${id}`);

export const createProject = (data: CreateProjectPayload) =>
  http.request<Project>("post", "/api/projects", { data });

export const updateProject = (id: number, data: UpdateProjectPayload) =>
  http.request<ProjectDetail>("put", `/api/projects/${id}`, { data });

export const deleteProject = (id: number) =>
  http.request<{ deleted: boolean }>("delete", `/api/projects/${id}`);

export const listTeams = () => http.request<Team[]>("get", "/api/teams");

export const createTeam = (name: string) =>
  http.request<Team>("post", "/api/teams", { data: { name } });

export const updateTeam = (id: number, name: string) =>
  http.request<Team>("put", `/api/teams/${id}`, { data: { name } });

export const deleteTeam = (id: number) =>
  http.request<{ deleted: boolean }>("delete", `/api/teams/${id}`);

export const listNodes = () => http.request<Node[]>("get", "/api/nodes");

export const createNode = (name: string, address: string, api_key: string) =>
  http.request<Node>("post", "/api/nodes", {
    data: { name, address, api_key }
  });

export interface UpdateNodePayload {
  name: string;
  address: string;
  api_key?: string;
}

export const updateNode = (id: number, data: UpdateNodePayload) =>
  http.request<Node>("put", `/api/nodes/${id}`, { data });

export const deleteNode = (id: number) =>
  http.request<{ deleted: boolean }>("delete", `/api/nodes/${id}`);

export const deploy = (project_id: number, compose_yaml: string) =>
  http.request<{ task_id: string; version: string; status: string }>(
    "post",
    "/api/deploy",
    {
      data: { project_id, compose_yaml }
    }
  );

export const listDeployments = (projectId: number, page = 1) =>
  http.request<DeploymentPage>(
    "get",
    `/api/projects/${projectId}/deployments`,
    { params: { page } }
  );

export const listAllDeployments = (page = 1) =>
  http.request<DeploymentPage>("get", "/api/deployments", {
    params: { page }
  });

export const rollbackDeployment = (id: number) =>
  http.request<{ task_id: string; version: string }>(
    "post",
    `/api/deployments/${id}/rollback`
  );

export const retryDeployment = (id: number) =>
  http.request<{ task_id: string; version: string }>(
    "post",
    `/api/deployments/${id}/retry`
  );

export const getDeploymentDetail = (id: number) =>
  http.request<DeploymentDetail>("get", `/api/deployments/${id}`);

export const approveDeployment = (id: number, comment: string) =>
  http.request<{ task_id: string; version: string; status: string }>(
    "post",
    `/api/deployments/${id}/approve`,
    { data: { comment } }
  );

export const rejectDeployment = (id: number, comment: string) =>
  http.request<{ status: string }>("post", `/api/deployments/${id}/reject`, {
    data: { comment }
  });

export function getApiErrorMessage(error: unknown, fallback: string): string {
  const response = (
    error as { response?: { data?: { error?: string } } } | undefined
  )?.response;
  return response?.data?.error ?? fallback;
}
