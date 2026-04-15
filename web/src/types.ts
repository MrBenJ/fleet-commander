export interface SquadronAgent {
  name: string;
  branch: string;
  prompt: string;
  driver: string;
  persona: string;
}

export interface SquadronData {
  name: string;
  consensus: "universal" | "review_master" | "none";
  reviewMaster?: string;
  baseBranch?: string;
  autoMerge: boolean;
  autoPR?: boolean;
  mergeMaster?: string;
  useJumpSh?: boolean;
  agents: SquadronAgent[];
}

export interface FleetInfo {
  repoPath: string;
  currentBranch: string;
  ghAvailable: boolean;
  agents: AgentInfo[];
}

export interface AgentInfo {
  name: string;
  branch: string;
  status: string;
  driver: string;
  hooksOK: boolean;
  stateFilePath: string;
}

export interface Persona {
  name: string;
  displayName: string;
  preamble: string;
}

export interface ContextMessage {
  agent: string;
  message: string;
  timestamp: string;
}

export type WSEvent =
  | { type: "context_message"; agent: string; message: string; timestamp: string }
  | { type: "agent_state"; agent: string; state: string; timestamp: string }
  | { type: "squadron_launched"; name: string; agents: string[] }
  | { type: "agent_stopped"; agent: string };
