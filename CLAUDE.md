# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Vespera-coze is an **agent orchestrator workspace** that coordinates and manages worker agent sessions for the `jssyxd` GitHub repository. This directory serves as the base for spawning, monitoring, and managing agent sessions.

**This is not a code repository itself** — it's a workspace that creates git worktrees for each session to work on issues from the target repository.

## Target Repository

- **Repository**: jssyxd (GitHub)
- **Default Branch**: main

## Orchestrator Commands

Use the `ao` CLI to manage sessions:

| Command | Description |
|---------|-------------|
| `ao status` | Show all sessions with PR/CI/review status |
| `ao spawn Vespera-coze [issue]` | Spawn a worker agent session for an issue |
| `ao batch-spawn Vespera-coze [issues...]` | Spawn multiple sessions in parallel |
| `ao session ls -p Vespera-coze` | List all sessions for this project |
| `ao session attach <session>` | Attach to a session's tmux window |
| `ao session kill <session>` | Kill a specific session |
| `ao session cleanup -p Vespera-coze` | Remove completed/merged sessions |
| `ao send <session> "message"` | Send message to a running session |
| `ao dashboard` | Start web dashboard (http://localhost:3011) |
| `ao open Vespera-coze` | Open all project sessions in terminal tabs |

## Session Management

When spawning a session:
1. Git worktree created from `main` branch
2. Feature branch created (e.g., `feat/INT-1234`)
3. Tmux session started (e.g., `vc-1`)
4. Agent launched with issue context
5. Metadata written to project sessions directory

Session status values: `working`, `pr_open`, `review_pending`, `merged`, `closed`

## Dashboard

Web dashboard available at **http://localhost:3011** with:
- Live session cards
- PR table with CI checks and review state
- Attention zones (merge ready, needs response, working, done)
- Real-time updates via SSE
