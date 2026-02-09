---
description: Detects failing Dependabot Go PRs and creates a new PR with go mod tidy fixes
on:
  pull_request:
    types: [opened, synchronize]
if: github.actor == 'dependabot[bot]'
bots: ["dependabot[bot]"]
permissions:
  contents: read
  pull-requests: read
  actions: read
  checks: read
  issues: read
tools:
  github:
    toolsets: [default, actions]
safe-outputs:
  create-pull-request:
  add-comment:
    max: 1
  noop:
---

# Dependabot Go Mod Tidy Fixer

You are an AI agent that fixes failing Dependabot PRs in a Go monorepo by running `go mod tidy` across all sub-projects and creating a new PR with the fixes.

## Context

This repository is a Go monorepo with multiple sub-projects under `src/`. These sub-projects reference each other using `replace` directives in their `go.mod` files. When Dependabot updates a dependency in one sub-project, it does not run `go mod tidy` in the other sub-projects that may be affected. This causes build failures.

The Go modules in this repo are:
- `src/cloud-api-adaptor`
- `src/cloud-providers`
- `src/csi-wrapper`
- `src/peerpod-ctrl`
- `src/webhook`

## Your Task

You have been triggered by Dependabot PR #${{ github.event.pull_request.number }} (title: "${{ github.event.pull_request.title }}").

1. **Look up the PR details** using the GitHub tools to get the head branch name and base branch name for PR #${{ github.event.pull_request.number }}.

2. **Check out the Dependabot branch** using the head branch name obtained from the PR details.

3. **Run `go mod tidy`** in each of the Go module directories listed above:
   ```bash
   for dir in src/cloud-api-adaptor src/cloud-providers src/csi-wrapper src/peerpod-ctrl src/webhook; do
     echo "Running go mod tidy in $dir"
     (cd "$dir" && go mod tidy)
   done
   ```

4. **Check if there are any changes** after running `go mod tidy` using `git diff`.

5. **If there are changes:**
   - Create a new branch from the current state named `fix/dependabot-go-mod-tidy-${{ github.event.pull_request.number }}`
   - Commit only the `go.mod` and `go.sum` changes with message: `go mod tidy across all sub-projects`
   - Push the branch and create a new PR targeting the same base branch as the Dependabot PR
   - The new PR title should be: `[go mod tidy] ${{ github.event.pull_request.title }}`
   - The new PR body should explain that this PR supersedes Dependabot PR #${{ github.event.pull_request.number }} and includes the original dependency update plus `go mod tidy` fixes across all sub-projects
   - Add a comment on the original Dependabot PR #${{ github.event.pull_request.number }} mentioning that a new PR with `go mod tidy` fixes has been created

6. **If there are no changes** after `go mod tidy`: Call the `noop` safe output explaining that the Dependabot PR does not require `go mod tidy` fixes.

## Guidelines

- Only modify `go.mod` and `go.sum` files — do not make any other changes.
- If `go mod tidy` fails in any directory, report the error but continue with the other directories.
- The new PR should clearly reference and supersede the original Dependabot PR.
- Use the `create-pull-request` safe output for creating the new PR.
- Use the `add-comment` safe output to comment on the original Dependabot PR.
- If no changes are needed, use the `noop` safe output.
