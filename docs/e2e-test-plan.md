# E2E Test Plan: Fleet Commander Hangar Web UI

**Date**: 2026-04-15
**Author**: e2e-test-plan agent
**Status**: Draft

## Overview

This plan details how to add end-to-end tests to the Fleet Commander hangar web UI using Playwright. The tests cover the full wizard flow (Setup → Agents → Persona → Review), the MissionControl view, and the critical constraint of validating squadron launch payloads without actually launching squadrons.

The plan is designed so that implementation can be parallelized across multiple Fleet Commander agents.

---

## 1. Playwright Setup

### 1.1 Installation

Install Playwright and its test runner in the `web/` directory:

```bash
cd web/
npm install -D @playwright/test
npx playwright install chromium
```

Only Chromium is needed for CI. Developers can optionally install Firefox and WebKit for local cross-browser testing.

### 1.2 File Structure

```
web/
├── playwright.config.ts          # Playwright configuration
├── e2e/
│   ├── fixtures/
│   │   ├── fleet-server.ts       # Custom test fixture: starts fleet hangar per worker
│   │   ├── fleet-data.ts         # Test data factories (agents, channels, config)
│   │   └── launch-interceptor.ts # Route interceptor for POST /api/squadron/launch
│   ├── helpers/
│   │   ├── schema-validator.ts   # SquadronData JSON schema validation
│   │   └── selectors.ts          # Centralized CSS/aria selectors
│   ├── wizard/
│   │   ├── setup-step.spec.ts    # SetupStep form tests
│   │   ├── agents-step.spec.ts   # AgentsStep (manual + AI generate) tests
│   │   ├── persona-step.spec.ts  # PersonaStep selection tests
│   │   ├── review-step.spec.ts   # ReviewStep editing + launch tests
│   │   └── wizard-flow.spec.ts   # Full wizard navigation + data persistence
│   ├── mission/
│   │   ├── agent-pills.spec.ts   # Agent pill rendering + status dots
│   │   ├── context-log.spec.ts   # Context log message display
│   │   ├── tooltips.spec.ts      # AgentTooltip hover interactions
│   │   ├── sort.spec.ts          # Agent sort functionality
│   │   └── multi-view.spec.ts    # MultiView toggle + grid layout
│   └── launch/
│       ├── payload-validation.spec.ts  # SquadronData schema assertions
│       └── launch-intercept.spec.ts    # Intercept + mock launch endpoint
├── package.json                  # (updated with playwright scripts)
└── ...
```

### 1.3 Configuration — `web/playwright.config.ts`

```typescript
import { defineConfig, devices } from '@playwright/test';

const BASE_PORT = 4300;

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 2 : 4,
  reporter: process.env.CI ? 'github' : 'html',
  timeout: 30_000,
  expect: { timeout: 5_000 },

  use: {
    // baseURL is set dynamically per worker in the fleet-server fixture
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],

  // No webServer block — server lifecycle is managed by the custom fixture
  // (see Section 2 for why this is necessary for parallel isolation)
});
```

**Why no `webServer` block**: Playwright's `webServer` config starts one server for all workers. We need one server per worker for isolation, so server lifecycle is managed in a custom test fixture instead.

### 1.4 npm Scripts

Add to `web/package.json`:

```json
{
  "scripts": {
    "test:e2e": "playwright test",
    "test:e2e:ui": "playwright test --ui",
    "test:e2e:headed": "playwright test --headed",
    "test:e2e:debug": "playwright test --debug"
  }
}
```

### 1.5 CI Integration

Add a new job to the GitHub Actions workflow (`.github/workflows/ci.yml`):

```yaml
e2e-tests:
  runs-on: ubuntu-latest
  needs: [build-verification]  # ensure Go binary builds first
  steps:
    - uses: actions/checkout@v4

    - uses: actions/setup-go@v5
      with:
        go-version: 'stable'

    - uses: actions/setup-node@v4
      with:
        node-version: '20'
        cache: 'npm'
        cache-dependency-path: web/package-lock.json

    - name: Build Go binary
      run: go build -o fleet ./cmd/fleet/

    - name: Install frontend deps
      run: cd web && npm ci

    - name: Install Playwright browsers
      run: cd web && npx playwright install --with-deps chromium

    - name: Run E2E tests
      run: cd web && npm run test:e2e
      env:
        FLEET_BINARY: ${{ github.workspace }}/fleet

    - uses: actions/upload-artifact@v4
      if: failure()
      with:
        name: playwright-report
        path: web/playwright-report/
        retention-days: 7
```

---

## 2. Test Architecture for Parallel Agents

### 2.1 The Isolation Problem

Multiple test workers (and potentially multiple Fleet Commander agents) run E2E tests simultaneously. Each must have:
- Its own `fleet hangar` server on a unique port
- Its own fleet directory (`.fleet/config.json`, `.fleet/context.json`)
- No shared state with other workers

### 2.2 Custom Test Fixture — `web/e2e/fixtures/fleet-server.ts`

The core fixture extends Playwright's `test` with a per-worker fleet hangar server:

```typescript
import { test as base, expect } from '@playwright/test';
import { spawn, ChildProcess } from 'child_process';
import { mkdtempSync, writeFileSync, mkdirSync } from 'fs';
import { tmpdir } from 'os';
import { join } from 'path';

const BASE_PORT = 4300;
const FLEET_BINARY = process.env.FLEET_BINARY || 'fleet';

type FleetFixtures = {
  serverPort: number;
  fleetDir: string;
  serverProcess: ChildProcess;
  baseURL: string;
};

export const test = base.extend<{}, FleetFixtures>({
  // Worker-scoped: one server per worker, shared across tests in that worker
  serverPort: [async ({}, use, workerInfo) => {
    await use(BASE_PORT + workerInfo.workerIndex);
  }, { scope: 'worker' }],

  fleetDir: [async ({}, use) => {
    const dir = mkdtempSync(join(tmpdir(), 'fleet-e2e-'));
    const fleetDir = join(dir, '.fleet');
    mkdirSync(fleetDir, { recursive: true });

    // Write minimal fleet config
    writeFileSync(join(fleetDir, 'config.json'), JSON.stringify({
      repoPath: dir,
      agents: [],
    }));

    await use(dir);
    // Cleanup handled by OS temp dir or explicit rm
  }, { scope: 'worker' }],

  serverProcess: [async ({ serverPort, fleetDir }, use) => {
    const proc = spawn(FLEET_BINARY, [
      'hangar',
      '--port', String(serverPort),
      '--no-open',
    ], {
      cwd: fleetDir,
      env: { ...process.env, HOME: fleetDir },
    });

    // Wait for server to be ready
    const baseURL = `http://localhost:${serverPort}`;
    await waitForServer(baseURL);

    await use(proc);

    proc.kill('SIGTERM');
  }, { scope: 'worker' }],

  baseURL: [async ({ serverPort }, use) => {
    await use(`http://localhost:${serverPort}`);
  }, { scope: 'worker' }],
});

async function waitForServer(url: string, timeoutMs = 15_000): Promise<void> {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    try {
      const res = await fetch(`${url}/api/health`);
      if (res.ok) return;
    } catch {}
    await new Promise(r => setTimeout(r, 200));
  }
  throw new Error(`Server at ${url} did not start within ${timeoutMs}ms`);
}

export { expect };
```

### 2.3 Port Allocation Strategy

| Worker Index | Port | Purpose |
|---|---|---|
| 0 | 4300 | First parallel worker |
| 1 | 4301 | Second parallel worker |
| 2 | 4302 | Third parallel worker |
| N | 4300+N | Nth worker |

This avoids port collisions. The `--port` flag on `fleet hangar` already exists in the CLI.

### 2.4 Fleet Data Seeding — `web/e2e/fixtures/fleet-data.ts`

Factory functions for creating test fleet configurations:

```typescript
export function createTestFleetConfig(agents: TestAgent[] = []): FleetConfig {
  return {
    repoPath: '/tmp/test-repo',
    agents: agents.map(a => ({
      name: a.name,
      branch: a.branch || `test-branch-${a.name}`,
      worktreePath: `/tmp/test-repo/.fleet/worktrees/${a.name}`,
      status: a.status || 'waiting',
      driver: a.driver || 'claude-code',
      hooksOK: true,
    })),
  };
}

export function createTestContextJson(channels: TestChannel[]): ContextJson {
  return {
    channels: Object.fromEntries(
      channels.map(ch => [ch.name, {
        members: ch.members,
        log: ch.messages.map((msg, i) => ({
          agent: msg.agent,
          message: msg.text,
          timestamp: new Date(Date.now() - (ch.messages.length - i) * 60000).toISOString(),
        })),
      }])
    ),
  };
}
```

### 2.5 Running Tests from Multiple Fleet Commander Agents

When multiple Fleet Commander agents run E2E tests in parallel:

1. Each agent runs `npm run test:e2e` from their own worktree
2. Playwright assigns unique worker indices within each process
3. Port ranges don't collide because each agent's Playwright instance manages its own workers from `BASE_PORT`
4. **If agents share the same machine**, use different `BASE_PORT` values via env var:
   ```bash
   BASE_PORT=4400 npm run test:e2e  # Agent 1
   BASE_PORT=4500 npm run test:e2e  # Agent 2
   ```
   Update the fixture to read `process.env.BASE_PORT || 4300`.

---

## 3. Launch Interception (DO NOT ACTUALLY LAUNCH)

This is the most critical section. Tests must validate the wizard produces correct SquadronData JSON without triggering `squadron.RunHeadless()`.

### 3.1 Route Interceptor — `web/e2e/fixtures/launch-interceptor.ts`

```typescript
import { Page } from '@playwright/test';

export interface CapturedLaunch {
  body: SquadronData;
  headers: Record<string, string>;
}

export async function interceptLaunch(page: Page): Promise<{
  waitForLaunch: () => Promise<CapturedLaunch>;
}> {
  let resolveLaunch: (value: CapturedLaunch) => void;
  const launchPromise = new Promise<CapturedLaunch>(resolve => {
    resolveLaunch = resolve;
  });

  await page.route('**/api/squadron/launch', async (route) => {
    const request = route.request();
    const body = JSON.parse(request.postData() || '{}');
    const headers = request.headers();

    resolveLaunch({ body, headers });

    // Return mock 204 — the server never sees this request
    await route.fulfill({
      status: 204,
      body: '',
    });
  });

  return {
    waitForLaunch: () => launchPromise,
  };
}
```

### 3.2 Schema Validator — `web/e2e/helpers/schema-validator.ts`

```typescript
export interface ValidationResult {
  valid: boolean;
  errors: string[];
}

const NAME_REGEX = /^[a-zA-Z0-9][a-zA-Z0-9_-]*$/;
const VALID_CONSENSUS = ['universal', 'review_master', 'none'] as const;
const VALID_DRIVERS = ['claude-code', 'codex', 'aider', 'kimi-code', 'generic'] as const;

export function validateSquadronData(data: unknown): ValidationResult {
  const errors: string[] = [];

  if (!data || typeof data !== 'object') {
    return { valid: false, errors: ['Payload is not an object'] };
  }

  const d = data as Record<string, unknown>;

  // Required: name
  if (typeof d.name !== 'string' || d.name.length === 0) {
    errors.push('name: required non-empty string');
  } else if (d.name.length > 30) {
    errors.push('name: must be 30 chars or fewer');
  } else if (!NAME_REGEX.test(d.name)) {
    errors.push('name: must match ^[a-zA-Z0-9][a-zA-Z0-9_-]*$');
  }

  // Required: consensus
  if (!VALID_CONSENSUS.includes(d.consensus as any)) {
    errors.push(`consensus: must be one of ${VALID_CONSENSUS.join(', ')}`);
  }

  // Conditional: reviewMaster
  if (d.consensus === 'review_master') {
    if (typeof d.reviewMaster !== 'string' || d.reviewMaster.length === 0) {
      errors.push('reviewMaster: required when consensus is review_master');
    }
  }

  // Required: autoMerge (boolean)
  if (typeof d.autoMerge !== 'boolean') {
    errors.push('autoMerge: must be a boolean');
  }

  // Required: agents (non-empty array)
  if (!Array.isArray(d.agents) || d.agents.length === 0) {
    errors.push('agents: required non-empty array');
  } else {
    const names = new Set<string>();
    (d.agents as any[]).forEach((agent, i) => {
      if (typeof agent.name !== 'string' || agent.name.length === 0) {
        errors.push(`agents[${i}].name: required`);
      } else {
        if (names.has(agent.name)) errors.push(`agents[${i}].name: duplicate "${agent.name}"`);
        names.add(agent.name);
      }
      if (typeof agent.branch !== 'string' || agent.branch.length === 0) {
        errors.push(`agents[${i}].branch: required`);
      }
      if (typeof agent.prompt !== 'string' || agent.prompt.length === 0) {
        errors.push(`agents[${i}].prompt: required`);
      }
    });
  }

  // Optional: mergeMaster must reference an agent
  if (d.mergeMaster && typeof d.mergeMaster === 'string') {
    const agentNames = (d.agents as any[] || []).map((a: any) => a.name);
    if (!agentNames.includes(d.mergeMaster)) {
      errors.push(`mergeMaster: "${d.mergeMaster}" is not in agents list`);
    }
  }

  return { valid: errors.length === 0, errors };
}
```

### 3.3 Test Pattern for Launch Validation

```typescript
// Example: web/e2e/launch/payload-validation.spec.ts
import { test, expect } from '../fixtures/fleet-server';
import { interceptLaunch } from '../fixtures/launch-interceptor';
import { validateSquadronData } from '../helpers/schema-validator';

test('wizard produces valid SquadronData on launch', async ({ page, baseURL }) => {
  const { waitForLaunch } = await interceptLaunch(page);

  await page.goto(baseURL!);

  // Walk through wizard...
  await page.fill('#squadron-name', 'test-squadron');
  await page.click('text=Continue →');
  // ... add agents, pick persona, review ...
  await page.click('text=Launch Squadron');

  const captured = await waitForLaunch();
  const result = validateSquadronData(captured.body);

  expect(result.valid).toBe(true);
  expect(result.errors).toEqual([]);
  expect(captured.body.name).toBe('test-squadron');
  expect(captured.body.agents.length).toBeGreaterThan(0);
});
```

---

## 4. Wizard Flow Testing

### 4.1 Centralized Selectors — `web/e2e/helpers/selectors.ts`

```typescript
export const SELECTORS = {
  // Setup Step
  squadronName: '#squadron-name',
  baseBranch: '#base-branch',
  continueButton: 'button:has-text("Continue →")',

  // Agents Step
  aiDescription: '#ai-description-label',
  generateButton: 'button:has-text("Generate Agent Breakdown")',
  manualName: '#manual-agent-name',
  manualBranch: '#manual-branch',
  manualPrompt: '#manual-prompt-label',
  manualHarness: '#manual-harness',
  manualPersona: '#manual-persona',
  addAgentButton: 'button:has-text("+ Add Agent")',
  agentList: '[aria-label="Agent list"]',

  // Persona Step
  personaList: '[aria-label="Available personas"]',
  backButton: 'button:has-text("← Back")',

  // Review Step
  agentCards: '[aria-label="Agents to launch"]',
  launchButton: 'button:has-text("Launch Squadron")',
  addMoreButton: 'button:has-text("+ Add More")',
  autoMergeCheckbox: '#auto-merge',

  // Mission Control
  squadronTitle: 'h1',
  statusBadge: '[role="status"]',
  agentNav: '[aria-label="Squadron agents"]',
  contextLog: '[aria-label="Squadron context messages"]',
} as const;
```

### 4.2 SetupStep Tests — `web/e2e/wizard/setup-step.spec.ts`

| Test Case | Description |
|---|---|
| `displays setup form on load` | Verify squadron name input and base branch dropdown are visible |
| `continue button disabled when name empty` | Assert button is disabled by default |
| `continue button enables on valid name` | Fill name, assert button becomes enabled |
| `populates branch dropdown from API` | Verify branches from `GET /api/fleet/branches` appear in select |
| `trims whitespace from squadron name` | Enter "  spaces  ", verify trimmed value enables continue |
| `navigates to agents step on continue` | Fill name, click continue, verify agents step is visible |

### 4.3 AgentsStep Tests — `web/e2e/wizard/agents-step.spec.ts`

| Test Case | Description |
|---|---|
| `shows manual add form and AI panel` | Both panels visible |
| `manual add: fills form and adds agent` | Fill name/prompt, click add, verify agent appears in list |
| `manual add: disabled when name or prompt empty` | Assert add button disabled state |
| `manual add: auto-generates branch from name` | Fill agent name, verify branch field auto-populates |
| `manual add: harness dropdown shows all drivers` | Mock `/api/fleet/drivers`, verify options |
| `AI generate: sends description and populates agents` | Mock `/api/squadron/generate`, fill description, click generate, verify agents added |
| `AI generate: shows loading state during generation` | Assert spinner/loading text appears |
| `edit agent in list` | Click edit on agent, modify fields, save |
| `remove agent from list` | Click remove, verify agent disappears |
| `persona selection via agent list` | Click "Pick Persona" link, select persona, verify assignment |

**AI Generate Mock Strategy:**
```typescript
await page.route('**/api/squadron/generate', async (route) => {
  await route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      agents: [
        { name: 'auth-agent', branch: 'feat/auth', prompt: 'Implement auth', driver: 'claude-code', persona: '' },
        { name: 'ui-agent', branch: 'feat/ui', prompt: 'Build UI', driver: 'claude-code', persona: '' },
      ],
    }),
  });
});
```

### 4.4 PersonaStep Tests — `web/e2e/wizard/persona-step.spec.ts`

| Test Case | Description |
|---|---|
| `displays all available personas` | Verify grid shows all personas from `/api/fleet/personas` |
| `each persona shows icon and description` | Check icon + name + flavor text for each |
| `selecting persona assigns it to agent` | Click persona, verify agent gets updated |
| `"No Persona" option clears persona` | Select "No Persona", verify persona field is empty |
| `back button returns to agents step` | Click back, verify agents step is visible with data intact |

### 4.5 ReviewStep Tests — `web/e2e/wizard/review-step.spec.ts`

| Test Case | Description |
|---|---|
| `displays all agents in cards` | Verify each agent card shows name, branch, harness, persona, prompt |
| `inline edit: activates edit mode` | Click edit, verify fields become editable |
| `inline edit: save persists changes` | Edit name, save, verify updated text |
| `inline edit: cancel discards changes` | Edit name, cancel, verify original text |
| `remove agent from review` | Click X, verify agent removed |
| `consensus selector: shows all options` | Universal, Single Reviewer, None visible |
| `consensus selector: review_master shows reviewer dropdown` | Select "Single Reviewer", verify dropdown appears with agent names |
| `auto-merge checkbox toggles` | Click checkbox, verify state changes |
| `launch button disabled when no agents` | Remove all agents, assert launch disabled |
| `"+ Add More" navigates back to agents step` | Click, verify agents step shown |

### 4.6 Full Wizard Flow Tests — `web/e2e/wizard/wizard-flow.spec.ts`

| Test Case | Description |
|---|---|
| `complete wizard flow: setup → agents → review → launch` | Walk through entire flow with valid data, verify launch payload |
| `data persists across step navigation` | Fill setup, add agents, go back to setup, verify name still there |
| `can navigate backward without losing data` | Setup → Agents → back to Setup → forward, verify agents preserved |
| `multiple agents with different personas` | Add 3 agents, assign different personas, verify all in launch payload |
| `review_master consensus includes reviewer in payload` | Select review_master, pick reviewer, verify in launch payload |

---

## 5. MissionControl View Testing

### 5.1 Test Data Setup

MissionControl tests require a running fleet with agents and context messages. The test fixture seeds:
- `.fleet/config.json` with 3-5 test agents in various states
- `.fleet/context.json` with a squadron channel containing test messages

```typescript
// Seed data for mission control tests
const missionAgents = [
  { name: 'alpha', status: 'working', driver: 'claude-code' },
  { name: 'beta', status: 'waiting', driver: 'codex' },
  { name: 'gamma', status: 'stopped', driver: 'aider' },
  { name: 'delta', status: 'starting', driver: 'claude-code' },
];

const missionChannel = {
  name: 'squadron-test',
  members: ['alpha', 'beta', 'gamma', 'delta'],
  messages: [
    { agent: 'alpha', text: 'COMPLETED: Finished auth module' },
    { agent: 'beta', text: 'Working on database schema' },
    { agent: 'gamma', text: 'APPROVED: alpha' },
  ],
};
```

### 5.2 Agent Pills Tests — `web/e2e/mission/agent-pills.spec.ts`

| Test Case | Description |
|---|---|
| `renders pill for each agent` | Count pills matches agent count |
| `pills show agent names` | Verify text content of each pill |
| `pills show correct status dots` | Working=green, waiting=orange, stopped=red, starting=gray |
| `pills show correct driver icons` | Claude Code icon, Codex icon, Aider icon |
| `merge master pill shows MERGE badge` | Verify badge visible on designated merger |
| `clicking pill navigates to agent` | Click pill, verify corresponding view updates |

### 5.3 Context Log Tests — `web/e2e/mission/context-log.spec.ts`

| Test Case | Description |
|---|---|
| `displays existing messages` | Verify seeded messages appear |
| `messages show timestamp and agent name` | Check format: "HH:MM agent-name: message" |
| `agent names are colored` | Verify each agent name has consistent color |
| `"● live" indicator appears` | Check for live indicator element |
| `new messages appear via WebSocket` | Send mock WebSocket event, verify new message renders |
| `auto-scrolls to bottom on new message` | Scroll up, trigger new message, verify scrolled to bottom |

**WebSocket Mock Strategy for New Messages:**

For tests that need WebSocket events, we can either:
1. Seed `.fleet/context.json` before the page loads (for initial state)
2. Write to `.fleet/context.json` during the test and wait for the 2-second poll cycle to broadcast

Option 2 is more realistic since it tests the actual polling mechanism:

```typescript
test('new messages appear via WebSocket', async ({ page, fleetDir }) => {
  // Navigate to mission control
  await page.goto(baseURL);
  // ... complete wizard or navigate to mission view ...

  // Write a new message to the context file
  const contextPath = join(fleetDir, '.fleet', 'context.json');
  const context = JSON.parse(readFileSync(contextPath, 'utf-8'));
  context.channels['squadron-test'].log.push({
    agent: 'alpha',
    message: 'NEW: Just deployed the fix',
    timestamp: new Date().toISOString(),
  });
  writeFileSync(contextPath, JSON.stringify(context));

  // Wait for WebSocket poll cycle (2s) + render
  await expect(page.getByText('Just deployed the fix')).toBeVisible({ timeout: 5000 });
});
```

### 5.4 Tooltip Tests — `web/e2e/mission/tooltips.spec.ts`

| Test Case | Description |
|---|---|
| `tooltip appears on agent pill hover` | Hover over pill, verify tooltip visible |
| `tooltip shows agent details` | Verify branch, driver, persona, status in tooltip |
| `tooltip shows agent prompt/task` | Verify prompt text displayed in read-only editor |
| `"Assume Control" button opens terminal` | Click button, verify navigation to `/terminal/{name}` |
| `"Stop" button shows confirmation dialog` | Click stop, verify confirmation appears |
| `tooltip disappears on mouse leave` | Move mouse away, verify tooltip hidden |

### 5.5 Sort Tests — `web/e2e/mission/sort.spec.ts`

| Test Case | Description |
|---|---|
| `default sort order` | Verify initial pill order |
| `sort by name (A-Z)` | Click sort option, verify alphabetical order |
| `sort by status` | Click sort option, verify waiting agents first (or expected grouping) |
| `sort persists during session` | Change sort, navigate away and back, verify sort maintained |

### 5.6 MultiView Tests — `web/e2e/mission/multi-view.spec.ts`

| Test Case | Description |
|---|---|
| `toggle switches between single and grid view` | Click toggle, verify view changes |
| `grid view shows iframe per agent` | Verify iframes with correct src attributes |
| `grid columns match agent count` | 2-4 agents = 2 cols, 5-9 = 3 cols, 10+ = 4 cols |
| `single view shows context log` | Verify context log visible in single view |
| `each grid panel shows agent name and driver icon` | Check panel headers |

---

## 6. Agent Task Decomposition

The E2E test implementation is broken into 5 parallelizable tasks. Each task is independent and can be assigned to a separate Fleet Commander agent.

### Task 1: Playwright Infrastructure Setup

**Branch**: `e2e/infrastructure`
**Files to create/modify**:
- `web/playwright.config.ts`
- `web/e2e/fixtures/fleet-server.ts`
- `web/e2e/fixtures/fleet-data.ts`
- `web/e2e/fixtures/launch-interceptor.ts`
- `web/e2e/helpers/schema-validator.ts`
- `web/e2e/helpers/selectors.ts`
- `web/package.json` (add scripts + devDependency)
- `.github/workflows/ci.yml` (add e2e job)

**Acceptance criteria**:
- `npm run test:e2e` command works (even if no tests yet, exits cleanly)
- Fleet server fixture starts and stops a real `fleet hangar` process
- Launch interceptor captures POST bodies correctly
- Schema validator passes for valid SquadronData and fails for invalid
- CI job is defined and would run on PR

**Estimated scope**: ~8 files, ~400 lines

### Task 2: Wizard Flow E2E Tests

**Branch**: `e2e/wizard-tests`
**Depends on**: Task 1 (fixtures and helpers)
**Files to create**:
- `web/e2e/wizard/setup-step.spec.ts`
- `web/e2e/wizard/agents-step.spec.ts`
- `web/e2e/wizard/persona-step.spec.ts`
- `web/e2e/wizard/review-step.spec.ts`
- `web/e2e/wizard/wizard-flow.spec.ts`

**Acceptance criteria**:
- All 4 wizard steps tested for form validation
- Navigation between steps verified
- Data persistence across steps verified
- Full end-to-end wizard flow passes
- AI generate endpoint is mocked (never calls real AI)

**Estimated scope**: ~5 files, ~500 lines

### Task 3: Launch Payload Validation Tests

**Branch**: `e2e/launch-tests`
**Depends on**: Task 1 (fixtures and helpers)
**Files to create**:
- `web/e2e/launch/payload-validation.spec.ts`
- `web/e2e/launch/launch-intercept.spec.ts`

**Acceptance criteria**:
- Launch endpoint is NEVER called for real (all intercepted)
- SquadronData JSON validated against schema for every launch test
- Edge cases tested: review_master with missing reviewer, empty agents, invalid names
- All consensus modes produce correct payload structure
- autoMerge and mergeMaster fields correctly included

**Estimated scope**: ~2 files, ~300 lines

### Task 4: MissionControl E2E Tests

**Branch**: `e2e/mission-tests`
**Depends on**: Task 1 (fixtures and helpers)
**Files to create**:
- `web/e2e/mission/agent-pills.spec.ts`
- `web/e2e/mission/context-log.spec.ts`
- `web/e2e/mission/tooltips.spec.ts`
- `web/e2e/mission/sort.spec.ts`
- `web/e2e/mission/multi-view.spec.ts`

**Acceptance criteria**:
- Agent pills render with correct status indicators
- Context log displays messages correctly
- Tooltips appear/disappear on hover without flickering
- Sort options reorder agent pills correctly
- MultiView toggle and grid layout work

**Estimated scope**: ~5 files, ~450 lines

### Task 5: Cross-Cutting & Error State Tests

**Branch**: `e2e/cross-cutting-tests`
**Depends on**: Task 1 (fixtures and helpers)
**Files to create**:
- `web/e2e/error-states.spec.ts`
- `web/e2e/theme.spec.ts`
- `web/e2e/responsive.spec.ts`

**Acceptance criteria**:
- API error responses show user-friendly error messages
- Theme toggle switches between light and dark
- Layout is usable at common viewport sizes
- WebSocket disconnection shows "DISCONNECTED" status
- Network failure during wizard shows error state (not silent failure)

**Estimated scope**: ~3 files, ~200 lines

### Task Dependency Graph

```
Task 1 (Infrastructure)
  ├── Task 2 (Wizard Tests)
  ├── Task 3 (Launch Tests)
  ├── Task 4 (MissionControl Tests)
  └── Task 5 (Cross-Cutting Tests)
```

Tasks 2–5 can all run in parallel once Task 1 is merged.

### Squadron Launch Configuration

To implement this plan via Fleet Commander, create a squadron with these agents:

```json
{
  "name": "e2e-impl",
  "consensus": "universal",
  "autoMerge": true,
  "agents": [
    {
      "name": "e2e-infra",
      "branch": "e2e/infrastructure",
      "prompt": "Implement Playwright infrastructure per docs/e2e-test-plan.md Task 1",
      "driver": "claude-code"
    },
    {
      "name": "e2e-wizard",
      "branch": "e2e/wizard-tests",
      "prompt": "Implement wizard flow E2E tests per docs/e2e-test-plan.md Task 2. Wait for e2e-infra to complete first.",
      "driver": "claude-code"
    },
    {
      "name": "e2e-launch",
      "branch": "e2e/launch-tests",
      "prompt": "Implement launch validation E2E tests per docs/e2e-test-plan.md Task 3. Wait for e2e-infra to complete first.",
      "driver": "claude-code"
    },
    {
      "name": "e2e-mission",
      "branch": "e2e/mission-tests",
      "prompt": "Implement MissionControl E2E tests per docs/e2e-test-plan.md Task 4. Wait for e2e-infra to complete first.",
      "driver": "claude-code"
    },
    {
      "name": "e2e-cross",
      "branch": "e2e/cross-cutting-tests",
      "prompt": "Implement cross-cutting E2E tests per docs/e2e-test-plan.md Task 5. Wait for e2e-infra to complete first.",
      "driver": "claude-code"
    }
  ]
}
```

---

## Appendix A: Key Selectors Reference

| Element | Selector | Step/View |
|---|---|---|
| Squadron name input | `#squadron-name` | Setup |
| Base branch select | `#base-branch` | Setup |
| Continue button | `button:has-text("Continue →")` | Setup |
| AI description editor | `#ai-description-label` | Agents |
| Generate button | `button:has-text("Generate Agent Breakdown")` | Agents |
| Manual agent name | `#manual-agent-name` | Agents |
| Manual branch | `#manual-branch` | Agents |
| Manual prompt | `#manual-prompt-label` | Agents |
| Manual harness | `#manual-harness` | Agents |
| Manual persona | `#manual-persona` | Agents |
| Add agent button | `button:has-text("+ Add Agent")` | Agents |
| Agent list | `[aria-label="Agent list"]` | Agents |
| Persona list | `[aria-label="Available personas"]` | Persona |
| Back button | `button:has-text("← Back")` | Persona |
| Agent cards | `[aria-label="Agents to launch"]` | Review |
| Launch button | `button:has-text("Launch Squadron")` | Review |
| Auto-merge checkbox | `#auto-merge` | Review |
| Status badge | `[role="status"]` | Mission |
| Agent nav | `[aria-label="Squadron agents"]` | Mission |
| Context log | `[aria-label="Squadron context messages"]` | Mission |

## Appendix B: API Endpoints Reference

| Method | Path | Purpose | Mock Strategy |
|---|---|---|---|
| GET | `/api/health` | Health check | Real (used by fixture to detect server ready) |
| GET | `/api/fleet` | Fleet info + agents | Real (seeded via config.json) |
| GET | `/api/fleet/personas` | Available personas | Real |
| GET | `/api/fleet/drivers` | Available drivers | Real |
| GET | `/api/fleet/branches` | Git branches | Real |
| POST | `/api/squadron/launch` | Launch squadron | **ALWAYS INTERCEPTED** — never real |
| POST | `/api/squadron/generate` | AI agent generation | **ALWAYS MOCKED** — avoid real AI calls |
| POST | `/api/agent/{name}/stop` | Stop agent | Mock or real depending on test |
| GET | `/ws/events` | WebSocket events | Real (drives mission control updates) |
