export type ConnectionType = "proxmox" | "homeassistant" | "adguard";

export interface AppState {
  setup_required: boolean;
  version: string;
  user: string | null;
}

export interface Connection {
  id: string;
  name: string;
  type: ConnectionType;
  base_url: string;
  username: string;
  interval_seconds: number;
  tls_skip_verify: boolean;
  options: Record<string, boolean> | null;
  entities: string[] | null;
  updated_at: string;
}

export interface ConnectionInput {
  name: string;
  type: ConnectionType;
  base_url: string;
  username: string;
  secret?: string;
  interval_seconds: number;
  tls_skip_verify: boolean;
  options: Record<string, boolean>;
  entities: string[];
}

export interface EntityOption {
  entity_id: string;
  name: string;
  state: string;
  unit?: string;
  device_class?: string;
}

export interface EntitiesResult {
  ok: boolean;
  error?: string;
  entities?: EntityOption[];
}

export interface HomeAssistantData {
  version: string;
  state: string;
  safe_mode: boolean;
  recovery_mode: boolean;
  entities: EntityOption[];
}

export interface DomainCount {
  domain: string;
  count: number;
}

export interface AdGuardData {
  version: string;
  running: boolean;
  protection_enabled: boolean;
  queries: number;
  blocked: number;
  blocked_percent: number;
  avg_processing_ms: number;
  period_unit: "hours" | "days" | "";
  period_length: number;
  top_queried?: DomainCount[];
  top_blocked?: DomainCount[];
}

export interface Count {
  running: number;
  total: number;
}

export interface ProxmoxGuest {
  vmid: number;
  name: string;
  type: "qemu" | "lxc";
  status: string;
  cpu: number;
  max_cpu: number;
  mem_used: number;
  mem_total: number;
}

export interface ProxmoxNode {
  name: string;
  online: boolean;
  cpu: number;
  max_cpu: number;
  mem_used: number;
  mem_total: number;
  disk_used: number;
  disk_total: number;
  vms: Count;
  containers: Count;
  guests?: ProxmoxGuest[];
}

export interface ProxmoxData {
  version: string;
  cluster_name?: string;
  quorate?: boolean;
  nodes_online: number;
  nodes_total: number;
  vms: Count;
  containers: Count;
  cpu: number;
  max_cpu: number;
  mem_used: number;
  mem_total: number;
  nodes: ProxmoxNode[];
}

export interface ConnectionState {
  id: string;
  name: string;
  type: ConnectionType;
  status: "pending" | "ok" | "error";
  error?: string;
  data: unknown;
  checked_at: string | null;
  last_success_at: string | null;
}

export interface DashboardState {
  connections: ConnectionState[];
}

export interface TestResult {
  ok: boolean;
  error?: string;
}

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
  ) {
    super(code);
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  let response: Response;
  try {
    response = await fetch(path, {
      method,
      credentials: "same-origin",
      headers: body === undefined ? undefined : { "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, "network");
  }
  if (response.status === 204) return undefined as T;
  const data = await response.json().catch(() => null);
  if (!response.ok) throw new ApiError(response.status, data?.error ?? "internal");
  return data as T;
}

export const api = {
  state: () => request<AppState>("GET", "/api/state"),
  setup: (username: string, password: string) => request("POST", "/api/setup", { username, password }),
  login: (username: string, password: string) => request("POST", "/api/auth/login", { username, password }),
  logout: () => request("POST", "/api/auth/logout"),
  connections: () => request<Connection[]>("GET", "/api/admin/connections"),
  createConnection: (input: ConnectionInput) => request<Connection>("POST", "/api/admin/connections", input),
  updateConnection: (id: string, input: ConnectionInput) =>
    request<Connection>("PUT", `/api/admin/connections/${encodeURIComponent(id)}`, input),
  testConnection: (input: ConnectionInput & { id?: string }) =>
    request<TestResult>("POST", "/api/admin/connections/test", input),
  listEntities: (input: ConnectionInput & { id?: string }) =>
    request<EntitiesResult>("POST", "/api/admin/connections/entities", input),
  dashboard: () => request<DashboardState>("GET", "/api/dashboard"),
  deleteConnection: (id: string) => request("DELETE", `/api/admin/connections/${encodeURIComponent(id)}`),
};
