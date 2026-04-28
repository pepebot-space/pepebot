# Task Orchestration & Kanban Board

## Table of Contents
- [Overview](#overview)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Kanban Board](#kanban-board)
- [Task Model](#task-model)
- [API Reference](#api-reference)
- [Agent Tool](#agent-tool)
- [Task Templates](#task-templates)
- [Auto-Dispatch](#auto-dispatch)
- [Approval Gates](#approval-gates)
- [Workflow Integration](#workflow-integration)
- [Channel Notifications](#channel-notifications)
- [WebSocket Real-time](#websocket-real-time)
- [Storage Backends](#storage-backends)
- [Architecture](#architecture)

---

## Overview

Task Orchestration adds a Kanban-based task management system to Pepebot, inspired by [Paperclip](https://github.com/paperclipai/paperclip)'s agent orchestration patterns. It enables:

- **Structured task management** — agents work on discrete tasks, not just chat messages
- **Kanban board** — visual drag & drop dashboard for managing task flow
- **Atomic task checkout** — prevents double-work when multiple agents run concurrently
- **Auto-dispatch** — heartbeat picks up unassigned tasks and routes to matching agents
- **Approval gates** — human-in-the-loop governance for critical tasks
- **Task templates** — reusable blueprints with variable interpolation and sub-tasks
- **Real-time updates** — WebSocket push events with polling fallback

### Key Features

| Feature | Description |
|---------|-------------|
| Dual storage backend | SQLite (default) + JSON file fallback for MIPS/embedded |
| Feature-flagged | Disabled by default, zero overhead when off |
| Agent-native | Agents create, claim, and complete tasks via `manage_task` tool |
| Dashboard integrated | Kanban board appears when orchestration is enabled |
| Workflow integration | `task` step type for creating and waiting on tasks |
| Channel notifications | Telegram/Discord alerts for approval requests |

---

## Quick Start

### 1. Enable Orchestration

Add to `~/.pepebot/config.json`:

```json
{
  "orchestration": {
    "enabled": true
  }
}
```

Or via environment variable:

```bash
export PEPEBOT_ORCHESTRATION_ENABLED=true
```

### 2. Restart Gateway

```bash
pepebot gateway
```

You'll see in the logs:
```
[INFO] task: Using SQLite backend
[INFO] agent: Task orchestration tool registered
[INFO] gateway: Task orchestration enabled
```

### 3. Open Dashboard

Navigate to `http://localhost:3000/#/tasks` — the Kanban board icon appears in the sidebar.

### 4. Create Your First Task

Via dashboard "New Task" button, or via agent:

```
You: create a task called "Setup CI pipeline" with high priority and labels devops,ci
```

The agent will use the `manage_task` tool to create the task on the Kanban board.

---

## Configuration

### Config Structure

```json
{
  "orchestration": {
    "enabled": false,
    "backend": "auto",
    "db_path": "~/.pepebot/tasks.db",
    "tasks_dir": "~/.pepebot/workspace/tasks",
    "ttl": {
      "done_days": 30,
      "failed_days": 7
    }
  }
}
```

### Config Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable/disable orchestration |
| `backend` | string | `"auto"` | Storage backend: `"auto"`, `"sqlite"`, `"json"` |
| `db_path` | string | `~/.pepebot/tasks.db` | SQLite database file path |
| `tasks_dir` | string | `~/.pepebot/workspace/tasks` | JSON backend directory |
| `ttl.done_days` | int | `30` | Auto-delete completed tasks after N days |
| `ttl.failed_days` | int | `7` | Auto-delete failed tasks after N days |

### Environment Variables

| Variable | Maps to |
|----------|---------|
| `PEPEBOT_ORCHESTRATION_ENABLED` | `orchestration.enabled` |
| `PEPEBOT_ORCHESTRATION_BACKEND` | `orchestration.backend` |
| `PEPEBOT_ORCHESTRATION_DB_PATH` | `orchestration.db_path` |
| `PEPEBOT_ORCHESTRATION_TASKS_DIR` | `orchestration.tasks_dir` |
| `PEPEBOT_ORCHESTRATION_TTL_DONE_DAYS` | `orchestration.ttl.done_days` |
| `PEPEBOT_ORCHESTRATION_TTL_FAILED_DAYS` | `orchestration.ttl.failed_days` |

---

## Kanban Board

The dashboard Kanban board at `/#/tasks` provides:

### Columns
| Column | Status | Description |
|--------|--------|-------------|
| Backlog | `backlog` | Ideas and future work |
| Todo | `todo` | Ready to be picked up |
| In Progress | `in_progress` | Currently being worked on |
| Review | `review` | Awaiting review or approval |
| Done | `done` | Completed |
| Failed | `failed` | Failed execution |

### Features
- **Drag & drop** — move tasks between columns
- **Create task** — modal with title, description, priority, agent, labels, approval toggle
- **Task detail** — slide-over panel with full info, approve/reject buttons, result/error display
- **Filters** — search, filter by agent, filter by priority
- **Auto-refresh** — WebSocket real-time updates with 5s polling fallback
- **Feature detection** — Kanban icon only appears when orchestration is enabled

---

## Task Model

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Auto-generated unique ID (`task-{timestamp}`) |
| `title` | string | Task title (required) |
| `description` | string | Detailed description |
| `status` | string | Current status (see columns above) |
| `priority` | string | `low`, `medium`, `high`, `critical` |
| `assigned_agent` | string | Agent assigned to this task |
| `created_by` | string | Who created the task |
| `labels` | string[] | Tags for categorization and agent matching |
| `parent_id` | string | Parent task ID (for sub-tasks) |
| `approval` | bool | Whether task requires explicit human approval |
| `result` | string | Completion result |
| `error` | string | Error message if failed |
| `ttl_days` | int | Per-task TTL override (null = use global) |
| `template_id` | string | Template this task was created from |

### Status Transitions

```
backlog     → todo
todo        → in_progress, backlog
in_progress → review, done, failed, todo
review      → done (if !approval), approved (if approval), rejected, in_progress, todo
approved    → done
rejected    → todo
done        → todo (reopen)
failed      → todo (retry)
```

### Priority Sort Order

Tasks are sorted by priority (critical first), then by creation time (oldest first):

```
critical (0) > high (1) > medium (2) > low (3)
```

---

## API Reference

All task endpoints return `404` with `{"error": "orchestration not enabled"}` when disabled.

### Feature Detection

```
GET /v1/features
```

```json
{
  "orchestration": {"enabled": true, "backend": "auto"},
  "live": {"enabled": false}
}
```

### Task CRUD

```
GET    /v1/tasks                    # List tasks
POST   /v1/tasks                    # Create task
GET    /v1/tasks/{id}               # Get task
PUT    /v1/tasks/{id}               # Update task
DELETE /v1/tasks/{id}               # Delete task
GET    /v1/tasks/stats              # Task counts by status
```

#### List Tasks

```
GET /v1/tasks?status=todo&agent=coder&priority=high&labels=code,test&limit=50&offset=0
```

Response:
```json
{
  "tasks": [...],
  "total": 5
}
```

#### Create Task

```
POST /v1/tasks
Content-Type: application/json

{
  "title": "Deploy v2.0",
  "description": "Deploy new version to production",
  "priority": "critical",
  "status": "todo",
  "assigned_agent": "coder",
  "labels": ["deploy", "prod"],
  "approval": true
}
```

### Task Actions

```
POST /v1/tasks/{id}/move       # Change status      {"status": "in_progress"}
POST /v1/tasks/{id}/checkout   # Atomic claim        {"agent": "coder", "agent_labels": ["code"]}
POST /v1/tasks/{id}/complete   # Mark done           {"result": "Deployed successfully"}
POST /v1/tasks/{id}/fail       # Mark failed         {"error": "Build failed"}
POST /v1/tasks/{id}/approve    # Approve             {"approved_by": "admin", "note": "LGTM"}
POST /v1/tasks/{id}/reject     # Reject              {"rejected_by": "admin", "note": "Needs tests"}
```

### Task Stats

```
GET /v1/tasks/stats
```

```json
{
  "backlog": 3, "todo": 5, "in_progress": 2,
  "review": 1, "done": 12, "failed": 0,
  "total": 23
}
```

### Templates

```
GET    /v1/task-templates              # List templates
GET    /v1/task-templates/{name}       # Get template
POST   /v1/task-templates              # Save template
DELETE /v1/task-templates/{name}       # Delete template
POST   /v1/task-templates/{name}/run   # Create tasks from template
```

---

## Agent Tool

Agents interact with the task system via the `manage_task` tool. This tool is automatically registered when orchestration is enabled.

### Actions

| Action | Description |
|--------|-------------|
| `create` | Create a new task |
| `list` | List tasks (with optional filters) |
| `get` | Get task by ID |
| `checkout` | Claim a task (specific ID or auto-assign next) |
| `complete` | Mark task as done |
| `fail` | Mark task as failed |
| `move` | Change task status |
| `stats` | Get task counts |
| `create_from_template` | Create tasks from a template |

### Examples

**Agent creates a task:**
```json
{
  "tool": "manage_task",
  "args": {
    "action": "create",
    "title": "Research competitor pricing",
    "priority": "high",
    "labels": ["research", "market"]
  }
}
```

**Agent checks out next available task:**
```json
{
  "tool": "manage_task",
  "args": {
    "action": "checkout",
    "agent": "researcher"
  }
}
```

**Agent completes task with result:**
```json
{
  "tool": "manage_task",
  "args": {
    "action": "complete",
    "id": "task-123456",
    "result": "Found 5 competitors with pricing data..."
  }
}
```

**Agent creates tasks from template:**
```json
{
  "tool": "manage_task",
  "args": {
    "action": "create_from_template",
    "template": "code-review",
    "variables": {"branch": "feat/login", "repo": "pepebot"}
  }
}
```

---

## Task Templates

Templates are reusable task blueprints stored as JSON files in `~/.pepebot/workspace/task-templates/`.

### Template Structure

```json
{
  "name": "Code Review",
  "description": "Review {{branch}} for {{repo}}",
  "priority": "high",
  "agent": "reviewer",
  "labels": ["review", "code"],
  "approval": true,
  "variables": {
    "branch": "",
    "repo": ""
  },
  "sub_tasks": [
    {
      "name": "Security Check for {{branch}}",
      "description": "Check for security vulnerabilities",
      "agent": "security-agent",
      "priority": "critical"
    },
    {
      "name": "Style Check for {{branch}}",
      "description": "Verify code style and formatting",
      "agent": "linter",
      "priority": "low"
    }
  ]
}
```

### Variable Interpolation

Variables use `{{variable_name}}` syntax and are replaced at creation time:

- **Template defaults**: defined in `variables` field
- **Runtime overrides**: passed when running the template
- **Runtime wins**: overrides take precedence over defaults

### Running a Template

Via API:
```bash
curl -X POST http://localhost:18790/v1/task-templates/code-review/run \
  -H "Content-Type: application/json" \
  -d '{"variables": {"branch": "feat/login", "repo": "pepebot"}}'
```

Via agent tool:
```json
{"action": "create_from_template", "template": "code-review", "variables": {"branch": "feat/login"}}
```

This creates a parent task + sub-tasks, all linked via `parent_id`.

---

## Auto-Dispatch

When orchestration is enabled, a background ticker (every 30 seconds) automatically:

1. **Scans** for unassigned `todo` tasks
2. **Matches** agents by `task_labels`
3. **Claims** the task atomically (prevents double-work)
4. **Executes** the task by sending it to the matched agent
5. **Records** the result or error

### Agent Task Labels

Restrict which tasks an agent can work on via `task_labels` in the agent registry:

```json
{
  "agents": {
    "coder": {
      "enabled": true,
      "model": "gemini-2.5-flash",
      "task_labels": ["code", "bugfix", "deploy"]
    },
    "researcher": {
      "enabled": true,
      "model": "gemini-2.5-flash",
      "task_labels": ["research", "analysis"]
    },
    "default": {
      "enabled": true,
      "model": "gemini-2.5-flash",
      "task_labels": []
    }
  }
}
```

**Matching rules:**
- Agent with `task_labels: []` (empty) can work on **any** task
- Agent with specific labels can only checkout tasks with **at least one matching label**
- If no agent matches a task's labels, the task stays in `todo`

### TTL Cleanup

The same ticker also cleans up expired tasks:
- Tasks in `done` status are deleted after `ttl.done_days` (default: 30)
- Tasks in `failed` status are deleted after `ttl.failed_days` (default: 7)

---

## Approval Gates

Tasks marked with `approval: true` require explicit human approval before completion.

### Flow

```
in_progress → review → [human approves] → done
                      → [human rejects]  → rejected → todo (retry)
```

### Creating Approval Tasks

Via API:
```json
{"title": "Deploy to production", "approval": true}
```

Via agent — when an approval task is "completed" by an agent, it moves to `review` instead of `done`.

### Approving/Rejecting

**Dashboard**: Click task in Review column → Approve/Reject buttons with optional note.

**API**:
```bash
# Approve
curl -X POST http://localhost:18790/v1/tasks/{id}/approve \
  -d '{"approved_by": "admin", "note": "LGTM"}'

# Reject
curl -X POST http://localhost:18790/v1/tasks/{id}/reject \
  -d '{"rejected_by": "admin", "note": "Needs more tests"}'
```

**Chat channel** (when notifications are enabled):
```
/approve task-123
/reject task-123 "needs more tests"
```

---

## Workflow Integration

Workflows support a `task` step type for creating and waiting on orchestration tasks.

### Task Step: Create

```json
{
  "name": "create_test_task",
  "task": {
    "action": "create",
    "title": "Run integration tests for {{branch}}",
    "agent": "tester",
    "priority": "high",
    "approval": false
  }
}
```

The created task's ID is available as `{{create_test_task_output}}`.

### Task Step: Wait

```json
{
  "name": "wait_for_tests",
  "task": {
    "action": "wait",
    "id": "{{create_test_task_output}}",
    "timeout": 600
  }
}
```

Polls the task store until the task reaches `done`, `failed`, or `rejected` status (or timeout).

### Full Workflow Example

```json
{
  "name": "deploy-pipeline",
  "description": "Run tests, get approval, deploy",
  "variables": {"branch": "main", "env": "production"},
  "steps": [
    {
      "name": "create_test_task",
      "task": {
        "action": "create",
        "title": "Test {{branch}} for {{env}}",
        "agent": "tester",
        "priority": "critical"
      }
    },
    {
      "name": "wait_tests",
      "task": {
        "action": "wait",
        "id": "{{create_test_task_output}}",
        "timeout": 300
      }
    },
    {
      "name": "deploy",
      "tool": "exec",
      "args": {"command": "make deploy ENV={{env}}"}
    }
  ]
}
```

---

## Channel Notifications

When tasks enter `review` with `approval: true`, notifications are sent to configured channels.

### Setup

Notifications are automatically enabled when:
1. Orchestration is enabled
2. Telegram or Discord channels are configured with `allow_from` chat IDs

### Notification Types

| Event | Message |
|-------|---------|
| Approval needed | Task title, agent, priority, approve/reject commands |
| Task completed | Task title, agent, result preview |
| Task failed | Task title, agent, error preview |

### Example Notification

```
🔔 Task needs approval

Deploy v2.0 to production
Agent: coder
Priority: critical

Reply /approve task-123 or /reject task-123 "reason"
```

---

## WebSocket Real-time

The dashboard connects to `ws://localhost:18790/v1/tasks/stream` for real-time task events.

### Connection

```javascript
const ws = new WebSocket('ws://localhost:18790/v1/tasks/stream')

ws.onmessage = (event) => {
  const data = JSON.parse(event.data)
  console.log(data.type, data.task)
}
```

### Event Types

| Event | Trigger |
|-------|---------|
| `task.created` | New task created |
| `task.moved` | Task status changed |
| `task.checkout` | Task claimed by agent |
| `task.completed` | Task marked done |
| `task.failed` | Task marked failed |
| `task.approved` | Task approved |
| `task.rejected` | Task rejected |

### Event Format

```json
{
  "type": "task.created",
  "task": { ... full task object ... },
  "time": "2026-04-02T12:00:00Z"
}
```

### Fallback

The dashboard starts with 5s polling, then switches to WebSocket when connected. If WebSocket disconnects, it falls back to polling and retries WS after 5 seconds.

---

## Storage Backends

### SQLite (default)

- File: `~/.pepebot/tasks.db`
- Uses `modernc.org/sqlite` (pure Go, no CGO)
- WAL mode for concurrent reads
- Indexes on status, agent, priority, parent_id, created_at
- Atomic checkout via transactions

### JSON (fallback)

- Directory: `~/.pepebot/workspace/tasks/`
- One JSON file per task
- In-memory map with file persistence
- Mutex-based atomic checkout
- No external dependencies — works on MIPS, ARM, any platform

### Backend Selection

| `backend` value | Behavior |
|----------------|----------|
| `"auto"` | Try SQLite, fallback to JSON on failure |
| `"sqlite"` | SQLite only, error if unavailable |
| `"json"` | JSON files only |

For MIPS/embedded devices:
```bash
export PEPEBOT_ORCHESTRATION_BACKEND=json
```

---

## Architecture

```
  Dashboard (Kanban)          Chat/CLI              Cron/Heartbeat
        │                        │                       │
        │ REST API               │ manage_task tool      │ Auto-dispatch
        ▼                        ▼                       ▼
  ┌──────────────────────────────────────────────────────────┐
  │                      Gateway Server                       │
  │  ┌─────────────┐  ┌──────────────┐  ┌────────────────┐  │
  │  │ Task API     │  │ TaskStreamHub │  │ Task Dispatcher │  │
  │  │ (REST)       │  │ (WebSocket)   │  │ (30s ticker)    │  │
  │  └──────┬───────┘  └──────┬───────┘  └───────┬────────┘  │
  │         │                  │                   │           │
  │         ▼                  ▼                   ▼           │
  │  ┌─────────────────────────────────────────────────────┐  │
  │  │              TaskStore Interface                     │  │
  │  │  ┌─────────────────┐  ┌──────────────────────────┐  │  │
  │  │  │ SQLiteTaskStore  │  │ JSONTaskStore             │  │  │
  │  │  │ (tasks.db)       │  │ (tasks/*.json)            │  │  │
  │  │  └─────────────────┘  └──────────────────────────┘  │  │
  │  └─────────────────────────────────────────────────────┘  │
  └──────────────────────────────────────────────────────────┘
```

### File Structure

```
pkg/task/
├── task.go             # Task model, statuses, transitions, validation
├── store.go            # TaskStore interface + factory
├── store_sqlite.go     # SQLite backend
├── store_json.go       # JSON file backend
├── store_test.go       # Tests (67 tests, both backends)
├── template.go         # Task templates
├── cleanup.go          # TTL cleanup
├── dispatch.go         # Auto-dispatch logic
├── workflow_bridge.go  # Workflow task step bridge
└── notify.go           # Channel notifications

pkg/gateway/
├── handlers_task.go      # Task REST API
├── handlers_template.go  # Template REST API
├── handlers_features.go  # Feature detection
└── taskstream.go         # WebSocket hub

pkg/tools/
└── manage_task.go        # Agent tool

dashboard/src/
├── views/Tasks.vue              # Kanban board
├── components/TaskCard.vue      # Draggable card
└── components/TaskDetail.vue    # Detail panel
```
