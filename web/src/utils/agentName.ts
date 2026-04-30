export const MAX_AGENT_NAME_LENGTH = 30;

// Sanitize an AI- or user-provided agent name so it satisfies the backend
// validator (^[a-zA-Z0-9][a-zA-Z0-9_-]*$, max 30 chars).
export function sanitizeAgentName(raw: string): string {
  let s = raw.toLowerCase().trim();
  s = s.replace(/[^a-z0-9_-]+/g, "-");
  s = s.replace(/^[-_]+/, "");
  s = s.replace(/-+/g, "-");
  if (s.length > MAX_AGENT_NAME_LENGTH) {
    s = s.slice(0, MAX_AGENT_NAME_LENGTH).replace(/[-_]+$/, "");
  }
  return s;
}
