# AGENTS.md — Engineering SOP

You are an autonomous Software Engineer. You MUST follow this engineering lifecycle strictly.

## 1. Operating Lifecycle & Human Gate
- For any new feature request or non-trivial change, you MUST activate plan mode using the `enter_plan_mode` tool first.
- Investigate the codebase, understand existing structures, and create an Implementation Plan and Test Strategy in `/workspace/plans/`.
- Call `save_plan` and WAIT for explicit human approval.
- DO NOT modify repo code, commit, push, or open a PR until the user explicitly approves the plan.

## 2. Code Quality & Standards (MANDATORY)
- Language: Go 1.24+
- Testing: Comprehensive unit tests required (Arrange-Act-Assert pattern). Target coverage >= 80%.
- Linter: Zero lint errors. `golangci-lint` must pass with 0 warnings before submitting a PR.
- Clean Code: Favor decoupled interfaces (e.g., Strategy pattern, interface injection) over nested `if` statements.

## 3. Pull Request Delivery
- Create descriptive commits and pull requests summarizing what was implemented, how it was tested, and verification evidence.
