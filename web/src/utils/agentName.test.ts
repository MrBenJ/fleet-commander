import { describe, expect, it } from "vitest";
import { MAX_AGENT_NAME_LENGTH, sanitizeAgentName } from "./agentName";

describe("sanitizeAgentName", () => {
  it("normalizes whitespace and punctuation into safe kebab case", () => {
    expect(sanitizeAgentName("  Build Login!!! Flow  ")).toBe("build-login-flow");
  });

  it("removes leading separators so backend validation accepts the name", () => {
    expect(sanitizeAgentName("___---alpha")).toBe("alpha");
  });

  it("trims long generated names without leaving trailing separators", () => {
    const result = sanitizeAgentName("frontend-component-builder-with-extra-words");
    expect(result.length).toBeLessThanOrEqual(MAX_AGENT_NAME_LENGTH);
    expect(result.endsWith("-")).toBe(false);
  });
});
