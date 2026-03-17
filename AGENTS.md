# AGENTS.md

This repository defines the release process for Istio. AI agents working on this project should understand the build/publish workflow and release management processes.

## Project Overview

The Istio release process is split into two phases:
1. **Build**: Pull sources, build artifacts, create manifest.yaml
2. **Publish**: Push to GCS, docker registries, tag repositories, create GitHub releases

## Development Environment

### Setup Commands

- Build locally: `make shell` (runs in Docker container with build tools)
- Test build: `go run main.go build --manifest example/manifest.yaml`
- Validate build: `go run main.go validate --release /tmp/istio-release/out`
- Docker container: Uses `gcr.io/istio-testing/build-tools` images

### Required Credentials

- `GITHUB_TOKEN` or `GH_TOKEN`: GitHub API access
- Docker credentials: For image publishing
- GCP credentials: For GCS publishing
- `GRAFANA_TOKEN`: For Grafana publishing

## Code Style & Patterns

- Go project following standard conventions
- Manifest-driven builds using YAML configuration
- Docker containerized builds for reproducibility
- Branch-based release workflow with automated PR creation

## Release Management Agent

When asked to check Istio release status, follow this process:

### 1. Get Release Information

- Ask for the Istio version to track (e.g., "1.29.0", "1.28.2"). You can also infer the version from the release tracking issue or wiki page if not provided.
- Calculate release branch as `release-<major>.<minor>` (e.g., "1.29.0" → `release-1.29`)
- Patch releases should also check the corresponding minor release branch (e.g., "1.28.2" → `release-1.28`). The changes for patch releases are included in the release branch, there is no separate branch for patch releases.

### 2. Check Core Repositories

Query these repositories for pending PRs on the release branch:
- istio/istio
- istio/api
- istio/ztunnel
- istio/proxy
- istio/client-go
- istio/tools
- istio/common-files
- istio/release-builder
- istio/enhancements

#### GitHub CLI Commands

```bash
# Get all open PRs targeting the release branch
gh search prs --base="release-<major>.<minor>" --state=open --owner="istio"

# Get PRs marked for backporting to release branch
gh search prs --base="master" --state=open --owner="istio" --label="cherrypick/release-<major>.<minor>"
```

### 3. Analyze Each Repository

For repositories with pending PRs, provide:
- Repository name
- Number of open PRs targeting release branch
- PR titles and links
- Review status (approved/changes requested/pending)
- Merge conflict status
- PR age (how long open)

### 4. Release Health Assessment

Calculate health status using timeline from release wiki page (pattern: `https://github.com/istio/istio/wiki/Istio-Release-<version>`):

**Health Indicators:**

- 🟢 **EXCELLENT**: 0-2 PRs, >1 week from milestone, no blockers
- 🟡 **GOOD**: 3-5 PRs, >3 days from milestone, minor issues
- 🟠 **MODERATE**: 6-10 PRs, <3 days from milestone, review delays
- 🔴 **CRITICAL**: >10 PRs, past deadline, major conflicts/blockers

**Timeline Context**:

- Get the exact dates for the release milestones from the wiki page, but a typical pattern is:
    - Branch Cut/Feature Freeze: Month date, year
    - Code Freeze/Release Candidate: Month date, year
    - Release Date: Month date, year

### 5. Output Format

Structure response should be minimal, we do not need to show overwhelming details, only show these sections:
- **Release Branch**: `release-<major>.<minor>`
- **Release Health Indicator**: Color-coded status with timeline
- **Repository Status**: List repos with pending PRs
- **PR list**: For each repo, show PR titles, links to access directly the PR, and review status
- **Release Timeline Context**: Current phase and milestones
- **Summary**: Overall status and recommendations for the release manager
- **Next Actions**: Suggested actions for release managers

Include references to:
- Release tracking GitHub issue (check wiki page)
- Release timeline wiki page
- Current date context (today: 2026-02-18)

## Branching Automation

**⚠️ CRITICAL SAFETY WARNING FOR AI AGENTS:**

- **NEVER execute branching commands automatically** - these are for human release managers only
- **NEVER access or read credentials/tokens** (GITHUB_TOKEN, GH_TOKEN, etc.)
- **ONLY provide documentation/guidance** when asked about branching steps
- **NEVER run `make shell`, Docker commands, or any branching scripts**
- Release branching affects the entire Istio ecosystem and must be done manually by a release manager with proper oversight and testing

**Human Release Manager Guidelines:**

For branch creation (manual process with automation steps 1-5):
- Each step creates PRs that need approval before proceeding
- Test against personal org forks before running on istio org
- Requires Docker buildx and proper credential setup

## Testing Instructions

- All builds run in containerized environment for consistency
- Validation step confirms build integrity
- Test branches should be created in personal forks first
- Watch for failure messages during automated steps

## Security Considerations

- **AI AGENTS**: Must never access, read, or use any credentials or tokens
- All credentials should be provided via environment variables (human operators only)
- Docker builds use verified base images from istio-testing
- GCS and container registry access requires proper GCP setup
- GitHub tokens need appropriate repository permissions
- Release operations are restricted to authorized human release managers

This AGENTS.md provides the context needed for AI agents to understand Istio release management and execute release status tracking effectively.
