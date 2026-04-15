import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { useFleet } from "./useFleet";
import * as api from "../api";

vi.mock("../api", () => ({
  getFleet: vi.fn(),
  getPersonas: vi.fn(),
  getDrivers: vi.fn(),
}));

const mockGetFleet = vi.mocked(api.getFleet);
const mockGetPersonas = vi.mocked(api.getPersonas);
const mockGetDrivers = vi.mocked(api.getDrivers);

beforeEach(() => {
  vi.resetAllMocks();
});

describe("useFleet", () => {
  it("starts in loading state", () => {
    mockGetFleet.mockReturnValue(new Promise(() => {})); // never resolves
    mockGetPersonas.mockReturnValue(new Promise(() => {}));
    mockGetDrivers.mockReturnValue(new Promise(() => {}));

    const { result } = renderHook(() => useFleet());
    expect(result.current.loading).toBe(true);
    expect(result.current.fleet).toBeNull();
    expect(result.current.error).toBeNull();
  });

  it("loads fleet, personas, and drivers", async () => {
    const fleet = { repoPath: "/repo", currentBranch: "main", agents: [] };
    const personas = [{ name: "zen", displayName: "Zen Master", preamble: "be calm" }];
    const drivers = [{ name: "claude-code" }, { name: "aider" }];

    mockGetFleet.mockResolvedValue(fleet);
    mockGetPersonas.mockResolvedValue(personas);
    mockGetDrivers.mockResolvedValue(drivers);

    const { result } = renderHook(() => useFleet());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.fleet).toEqual(fleet);
    expect(result.current.personas).toEqual(personas);
    expect(result.current.drivers).toEqual(["claude-code", "aider"]);
    expect(result.current.error).toBeNull();
  });

  it("sets error on failure", async () => {
    mockGetFleet.mockRejectedValue(new Error("network error"));
    mockGetPersonas.mockResolvedValue([]);
    mockGetDrivers.mockResolvedValue([]);

    const { result } = renderHook(() => useFleet());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe("network error");
    expect(result.current.fleet).toBeNull();
  });

  it("handles non-Error throws", async () => {
    mockGetFleet.mockRejectedValue("oops");
    mockGetPersonas.mockResolvedValue([]);
    mockGetDrivers.mockResolvedValue([]);

    const { result } = renderHook(() => useFleet());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe("Failed to load fleet");
  });
});
