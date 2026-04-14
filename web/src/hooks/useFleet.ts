import { useEffect, useState } from "react";
import type { FleetInfo, Persona } from "../types";
import { getFleet, getPersonas, getDrivers } from "../api";

export function useFleet() {
  const [fleet, setFleet] = useState<FleetInfo | null>(null);
  const [personas, setPersonas] = useState<Persona[]>([]);
  const [drivers, setDrivers] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function load() {
      try {
        const [f, p, d] = await Promise.all([
          getFleet(),
          getPersonas(),
          getDrivers(),
        ]);
        setFleet(f);
        setPersonas(p);
        setDrivers(d.map((x) => x.name));
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load fleet");
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);

  return { fleet, personas, drivers, loading, error };
}
