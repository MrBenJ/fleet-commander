import type {
  FleetInfo,
  Persona,
  SquadronData,
  SquadronAgent,
} from "./types";

const BASE = "";

async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(`${BASE}${path}`, {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(body.error || resp.statusText);
  }
  return resp.json();
}

export async function getFleet(): Promise<FleetInfo> {
  return fetchJSON("/api/fleet");
}

export async function getPersonas(): Promise<Persona[]> {
  return fetchJSON("/api/fleet/personas");
}

export async function getDrivers(): Promise<{ name: string }[]> {
  return fetchJSON("/api/fleet/drivers");
}

export async function getBranches(): Promise<string[]> {
  return fetchJSON("/api/fleet/branches");
}

export async function launchSquadron(
  data: SquadronData
): Promise<void> {
  const resp = await fetch("/api/squadron/launch", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(body.error || resp.statusText);
  }
}

export async function generateAgents(
  description: string
): Promise<{ agents: SquadronAgent[] }> {
  return fetchJSON("/api/squadron/generate", {
    method: "POST",
    body: JSON.stringify({ description }),
  });
}

export async function stopAgent(
  name: string
): Promise<{ status: string; agent: string }> {
  return fetchJSON(`/api/agent/${name}/stop`, { method: "POST" });
}
