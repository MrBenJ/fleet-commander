import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  getFleet,
  getPersonas,
  getDrivers,
  getAvailableDrivers,
  getBranches,
  launchSquadron,
  generateAgents,
  stopAgent,
} from "./api";
import type { SquadronData } from "./types";

const mockFetch = vi.fn();
globalThis.fetch = mockFetch;

function jsonResponse(data: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: status === 200 ? "OK" : "Internal Server Error",
    json: () => Promise.resolve(data),
  };
}

beforeEach(() => {
  mockFetch.mockReset();
});

describe("getFleet", () => {
  it("fetches fleet info from /api/fleet", async () => {
    const fleet = { repoPath: "/repo", currentBranch: "main", agents: [] };
    mockFetch.mockResolvedValue(jsonResponse(fleet));

    const result = await getFleet();
    expect(result).toEqual(fleet);
    expect(mockFetch).toHaveBeenCalledWith("/api/fleet", expect.objectContaining({
      headers: { "Content-Type": "application/json" },
    }));
  });

  it("throws on non-ok response", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ error: "not found" }, 404));
    await expect(getFleet()).rejects.toThrow("not found");
  });

  it("throws statusText when error body has no error field", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      statusText: "Internal Server Error",
      json: () => Promise.reject(new Error("bad json")),
    });
    await expect(getFleet()).rejects.toThrow("Internal Server Error");
  });
});

describe("getPersonas", () => {
  it("fetches personas from /api/fleet/personas", async () => {
    const personas = [{ name: "zen", displayName: "Zen", preamble: "..." }];
    mockFetch.mockResolvedValue(jsonResponse(personas));

    const result = await getPersonas();
    expect(result).toEqual(personas);
    expect(mockFetch).toHaveBeenCalledWith("/api/fleet/personas", expect.objectContaining({
      headers: { "Content-Type": "application/json" },
    }));
  });
});

describe("getDrivers", () => {
  it("fetches drivers from /api/fleet/drivers", async () => {
    const drivers = [{ name: "claude-code" }, { name: "aider" }];
    mockFetch.mockResolvedValue(jsonResponse(drivers));

    const result = await getDrivers();
    expect(result).toEqual(drivers);
  });
});

describe("getAvailableDrivers", () => {
  it("fetches runtime driver availability from /api/drivers/available", async () => {
    const drivers = [{ name: "claude-code", available: true }, { name: "codex", available: false }];
    mockFetch.mockResolvedValue(jsonResponse(drivers));

    const result = await getAvailableDrivers();
    expect(result).toEqual(drivers);
    expect(mockFetch).toHaveBeenCalledWith("/api/drivers/available", expect.objectContaining({
      headers: { "Content-Type": "application/json" },
    }));
  });
});

describe("getBranches", () => {
  it("fetches branches from /api/fleet/branches", async () => {
    const branches = ["main", "dev"];
    mockFetch.mockResolvedValue(jsonResponse(branches));

    const result = await getBranches();
    expect(result).toEqual(branches);
  });
});

describe("launchSquadron", () => {
  const squadronData: SquadronData = {
    name: "test-squad",
    consensus: "universal",
    autoMerge: false,
    agents: [{ name: "a1", branch: "b1", prompt: "do stuff", driver: "claude-code", persona: "zen" }],
  };

  it("posts squadron data to /api/squadron/launch", async () => {
    mockFetch.mockResolvedValue(jsonResponse({}));

    await launchSquadron(squadronData);
    expect(mockFetch).toHaveBeenCalledWith("/api/squadron/launch", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(squadronData),
    });
  });

  it("throws on non-ok response", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ error: "launch failed" }, 500));
    await expect(launchSquadron(squadronData)).rejects.toThrow("launch failed");
  });
});

describe("generateAgents", () => {
  it("posts description and returns agents", async () => {
    const agents = { agents: [{ name: "gen1", branch: "b", prompt: "p", driver: "claude-code", persona: "" }] };
    mockFetch.mockResolvedValue(jsonResponse(agents));

    const result = await generateAgents("build a thing", "codex");
    expect(result).toEqual(agents);
    expect(mockFetch).toHaveBeenCalledWith("/api/squadron/generate", expect.objectContaining({
      method: "POST",
      body: JSON.stringify({ description: "build a thing", driver: "codex" }),
    }));
  });
});

describe("stopAgent", () => {
  it("posts to stop endpoint and returns result", async () => {
    const response = { status: "stopped", agent: "myagent" };
    mockFetch.mockResolvedValue(jsonResponse(response));

    const result = await stopAgent("myagent");
    expect(result).toEqual(response);
    expect(mockFetch).toHaveBeenCalledWith("/api/agent/myagent/stop", expect.objectContaining({
      method: "POST",
    }));
  });
});
