import type { SquadronAgent } from "../types";

const VALID_DRIVERS = ["claude-code", "codex", "aider", "kimi-code", "generic"];
const VALID_PERSONAS = [
  "",
  "overconfident-engineer",
  "zen-master",
  "paranoid-perfectionist",
  "raging-jerk",
  "peter-molyneux",
];

const EXPECTED_HEADERS = ["agent_name", "branch", "prompt", "harness", "persona"];

export interface CSVParseError {
  row: number;
  message: string;
}

export interface CSVParseResult {
  agents: SquadronAgent[];
  errors: CSVParseError[];
}

/** Split a CSV line respecting quoted fields. */
function splitCSVLine(line: string): string[] {
  const fields: string[] = [];
  let current = "";
  let inQuotes = false;

  for (let i = 0; i < line.length; i++) {
    const ch = line[i];
    if (ch === '"') {
      if (inQuotes && line[i + 1] === '"') {
        current += '"';
        i++;
      } else {
        inQuotes = !inQuotes;
      }
    } else if (ch === "," && !inQuotes) {
      fields.push(current.trim());
      current = "";
    } else {
      current += ch;
    }
  }
  fields.push(current.trim());
  return fields;
}

export function parseAgentsCSV(
  text: string,
  squadronName: string,
): CSVParseResult {
  const lines = text.split(/\r?\n/).filter((l) => l.trim() !== "");
  if (lines.length === 0) {
    return { agents: [], errors: [{ row: 0, message: "CSV file is empty" }] };
  }

  const headerFields = splitCSVLine(lines[0]).map((h) => h.toLowerCase());
  const headerMismatch = EXPECTED_HEADERS.some((h, i) => headerFields[i] !== h);
  if (headerFields.length < EXPECTED_HEADERS.length || headerMismatch) {
    return {
      agents: [],
      errors: [
        {
          row: 1,
          message: `Invalid headers. Expected: ${EXPECTED_HEADERS.join(",")}`,
        },
      ],
    };
  }

  const agents: SquadronAgent[] = [];
  const errors: CSVParseError[] = [];

  for (let i = 1; i < lines.length; i++) {
    const fields = splitCSVLine(lines[i]);
    const rowNum = i + 1;

    const name = fields[0] || "";
    const branch = fields[1] || "";
    const prompt = fields[2] || "";
    const harness = fields[3] || "";
    const persona = fields[4] || "";

    if (!name) {
      errors.push({ row: rowNum, message: "agent_name is required" });
      continue;
    }

    if (!prompt) {
      errors.push({ row: rowNum, message: `Agent "${name}": prompt is required` });
      continue;
    }

    const driver = harness || "claude-code";
    if (!VALID_DRIVERS.includes(driver)) {
      errors.push({
        row: rowNum,
        message: `Agent "${name}": invalid harness "${harness}". Valid values: ${VALID_DRIVERS.join(", ")}`,
      });
      continue;
    }

    if (persona && !VALID_PERSONAS.includes(persona)) {
      errors.push({
        row: rowNum,
        message: `Agent "${name}": invalid persona "${persona}". Valid values: ${VALID_PERSONAS.filter(Boolean).join(", ")}`,
      });
      continue;
    }

    agents.push({
      name,
      branch: branch || `squadron/${squadronName}/${name}`,
      prompt,
      driver,
      persona,
    });
  }

  return { agents, errors };
}
