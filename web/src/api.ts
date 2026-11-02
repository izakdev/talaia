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
  deleteConnection: (id: string) => request("DELETE", `/api/admin/connections/${encodeURIComponent(id)}`),
};
