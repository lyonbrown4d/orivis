import axios, { AxiosError } from 'axios';

export type Status = 'up' | 'down' | 'degraded' | 'unknown' | string;

export interface DashboardSnapshot {
  name: string;
  env: string;
  generated_at: string;
  auth_enabled: boolean;
  all_monitors: number;
  group_slug: string;
  selected_group: string;
  summary: { agents: number; monitors: number; up: number; down: number; unknown: number };
  database: { driver: string; dialect?: string };
  groups: Array<{ name: string; slug: string; count: number; up: number; down: number; unknown: number; active: boolean }>;
  agents: Array<{ id: string; name: string; region_code: string; environment_codes: string[]; runtime_type: string; version: string; last_seen_at: string; status: string }>;
  monitors: Monitor[];
  recent_results: Result[];
  status_lights: Array<{ monitor_name: string; status: Status; latency_ms: number; checked_at: string }>;
}

export interface Monitor {
  id: string;
  name: string;
  type: string;
  target: string;
  group_name: string;
  environment_code: string;
  enabled: boolean;
  interval_ms: number;
  timeout_ms: number;
  retry_count: number;
  aggregation_policy: string;
  source: string;
  discovery_source: string;
  discovery_detail: string;
  latest?: Result;
}

export interface Result {
  id: string;
  monitor_id: string;
  monitor_name: string;
  agent_id: string;
  agent_name: string;
  region_code: string;
  environment_code: string;
  group_name: string;
  status: Status;
  latency_ms: number;
  error_message?: string;
  checked_at: string;
  created_at: string;
}

export interface AuthSession {
  authenticated: boolean;
  username?: string;
}

export const http = axios.create({
  baseURL: '/',
  withCredentials: true,
  headers: { 'Content-Type': 'application/json' }
});

const snapshotCache = new Map<string, { etag: string; data: DashboardSnapshot }>();

export async function login(username: string, password: string) {
  await http.post('/api/auth/login', { username, password });
}

export async function logout() {
  await http.post('/api/auth/logout');
}

export async function fetchSnapshot(group?: string): Promise<DashboardSnapshot> {
  const cacheKey = snapshotCacheKey(group);
  const cached = snapshotCache.get(cacheKey);
  const response = await http.get<DashboardSnapshot>('/api/dashboard/snapshot', {
    params: group ? { group } : undefined,
    headers: cached ? { 'If-None-Match': cached.etag } : undefined,
    validateStatus: (status) => (status >= 200 && status < 300) || status === 304
  });
  if (response.status === 304) {
    if (cached) {
      return cached.data;
    }
    throw new Error('dashboard snapshot cache missing for 304 response');
  }
  const etag = response.headers.etag;
  if (typeof etag === 'string' && etag !== '') {
    snapshotCache.set(cacheKey, { etag, data: response.data });
  } else {
    snapshotCache.delete(cacheKey);
  }
  return response.data;
}

function snapshotCacheKey(group?: string) {
  return group && group.trim() !== '' ? group : '__all__';
}

export async function fetchAuthSession(): Promise<AuthSession> {
  const response = await http.get<AuthSession>('/api/auth/me');
  return response.data;
}

export function isUnauthorized(error: unknown) {
  return axios.isAxiosError(error) && error.response?.status === 401;
}

export function errorMessage(error: unknown, fallback: string) {
  if (error instanceof AxiosError && typeof error.response?.data === 'object') {
    const data = error.response.data as { detail?: string; message?: string };
    return data.detail || data.message || fallback;
  }
  return error instanceof Error ? error.message : fallback;
}
