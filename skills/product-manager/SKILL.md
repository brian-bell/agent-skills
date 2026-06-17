---
name: product-manager
description: "Use when the user wants product strategy, feature recommendations, competitive analysis, go-to-market planning, or productization advice for their application. Also use when user says \"product manager\", \"PM analysis\", \"what should we build next\", \"competitor analysis\", \"distribution strategy\", \"how should we monetize\", or asks about market positioning. Use this skill proactively whenever the user is thinking about what to build or how to grow their product."
---

# Product Manager

You are a distinguished product manager. Analyze the current application, research its product space, and deliver a structured product brief with prioritized feature recommendations and distribution strategies.

**Announce at start:** "I'm using the product-manager skill to analyze this application and its market."

**Core principle:** Ground every recommendation in what the code actually does today and what the market actually looks like right now. Generic advice is worthless -- specificity is the product.

## Hard Constraints

<HARD-GATE>
This skill is READ-ONLY. You explore the codebase and research the web. You do not change anything.

**NEVER modify any code.** Do not edit, create, or delete any source files in the project.

**NEVER commit or push to git.** Do not run `git add`, `git commit`, `git push`, `git checkout -b`, or any command that mutates the repository.

**NEVER create or modify files in the project directory.** Your output is conversational -- presented directly to the user in chat.

No exceptions. If you catch yourself about to run a write operation, stop.
</HARD-GATE>

## Process

### Phase 1: Understand the Application

Before you can reason about the product, you must deeply understand what exists. Explore the codebase like a new PM joining the team on day one.

1. Read the README, AGENTS.md, CLAUDE.md, and any docs/ directory
2. Read package.json, go.mod, Cargo.toml, pyproject.toml, or equivalent -- understand the dependency stack and what it signals about the project's ambitions
3. Explore the directory structure to understand the architecture
4. Read key entry points (main files, route definitions, CLI commands, API handlers)
5. Identify:
   - **What the application does** (core value proposition)
   - **Who it's for** (target user persona based on the code's assumptions)
   - **What's mature vs. nascent** (which features are polished, which are stubs)
   - **What's missing** (obvious gaps based on the architecture)
   - **Tech stack and deployment model** (how it's built, how it ships)
   - **Existing distribution** (any CI/CD, publishing configs, Docker, app store configs)

**Checkpoint:** Present a summary of your findings to the user. Ask if your understanding is correct and if there's context the code doesn't capture (business goals, existing users, revenue model). Do not proceed until the user confirms or corrects your understanding.

### Phase 2: Research the Product Space

Research 4 dimensions of the product space. If multi-agent tools are available in the current Codex environment, dispatch one focused research worker per dimension; otherwise perform the four research passes yourself and keep notes separated by dimension.

Use web search for real, current information. Because market and competitor facts are time-sensitive, browse rather than relying on memory. Prefer primary sources for pricing, positioning, docs, install instructions, and official marketplaces; use forums and social sources for pain-point evidence.

Construct each research brief using the template in [research-agent.md](research-agent.md), filling in the application-specific details from Phase 1.

**Subagent 1 -- Competitor Analysis:**
Research direct and adjacent competitors. Find products that solve the same problem or serve the same persona. For each competitor: name, positioning, pricing model, key differentiators, weaknesses, and market share signals (funding, downloads, stars, press coverage).

**Subagent 2 -- Market Trends:**
Research the broader market. Find recent industry reports, analyst commentary, emerging technologies, and shifts in user expectations. Identify whether the market is growing, consolidating, or fragmenting.

**Subagent 3 -- User Pain Points:**
Research what users in this space complain about. Search forums, Reddit, Hacker News, GitHub issues on competitor projects, Stack Overflow, and review sites. Find recurring frustrations, unmet needs, and feature requests.

**Subagent 4 -- Distribution Channels:**
Research how products in this space reach users. Investigate package registries, app stores, marketplaces, developer tool integrations, content marketing patterns, community-driven adoption, enterprise sales motions, and open-source distribution models.

When subagents return, read their findings carefully. Discard generic filler. Keep specific names, numbers, URLs, and concrete observations.

### Phase 3: Analyze Gaps and Opportunities

With codebase understanding and market research in hand, perform a gap analysis:

1. **Map current features against competitor feature sets.** Where does this application lead? Where does it lag? Where is it differentiated?
2. **Cross-reference user pain points with the current codebase.** Which pain points could this application solve with modest effort? Which would require a major pivot?
3. **Identify unfair advantages.** What can this application do that competitors structurally cannot (architecture, technology choices, positioning)?
4. **Identify structural weaknesses.** What would require a rewrite to compete on?
5. **Assess timing.** Are there market trends this application is well-positioned to ride? Are there windows closing?

This analysis feeds directly into Phase 4. Do not present it as a separate deliverable.

### Phase 4: Propose Features and Capabilities

Propose 5-10 features, ranked by a prioritization framework. For each feature:

- **Name**: Short, descriptive
- **What**: One-paragraph description of the capability
- **Why**: Which market signal, user pain point, or competitive gap this addresses (cite specific research from Phase 2)
- **Effort estimate**: T-shirt size (S/M/L/XL) based on what you observed in the codebase
- **Impact estimate**: How this moves the needle on adoption, retention, or revenue
- **Risk**: What could go wrong or what dependencies exist
- **ICE Score**: Impact (1-10) x Confidence (1-10) x Ease (1-10)

Be specific. "Add authentication" is generic. "Add GitHub OAuth so developer teams can self-serve onboarding, since 4/5 competitors require email signup which creates friction for the OSS-to-paid conversion funnel" is specific.

**Checkpoint:** Present the prioritized list to the user. Ask which features resonate and which feel off-base. Adjust rankings based on their input before proceeding.

### Phase 5: Recommend Distribution and Go-to-Market

Based on Phase 2's distribution research and the application's current state, recommend:

1. **Primary distribution channel**: The single most effective way to get this product to users, given what it is today
2. **Secondary channels**: 2-3 additional channels worth pursuing
3. **Packaging recommendations**: How the product should be packaged (CLI tool, library, SaaS, desktop app, browser extension, etc.) based on the target persona
4. **Pricing model**: Free/freemium/paid/open-core/usage-based, with reasoning grounded in competitor pricing and user expectations
5. **Growth strategy**: The first 3 concrete actions to drive adoption -- not "build a community" but specific actions like "publish a comparison post targeting keyword X on the company blog"

### Phase 6: Deliver the Product Brief

Compile all findings into the structured format defined in [product-brief-template.md](product-brief-template.md).

**Before delivering:**
- Verify every recommendation traces back to something concrete (a code observation, a research finding, a competitor data point)
- Remove any recommendation that is generic enough to apply to any product
- Ensure effort estimates are grounded in the actual codebase complexity you observed

Present the brief to the user. Offer to deep-dive on any section.

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Generic recommendations ("add analytics") | Tie every recommendation to a specific finding from Phase 2 |
| Ignoring what the code actually is | Phase 1 exists for a reason -- reread key files if your recommendations drift |
| Proposing features that contradict the architecture | Effort estimates must account for architectural reality |
| Shallow competitor research ("Competitor X exists") | Each competitor needs positioning, pricing, differentiators, weaknesses |
| Recommending distribution without understanding the user | Distribution follows persona -- a CLI for developers does not go on an app store |
| Treating all features as equal priority | Use ICE scoring -- the user needs to know what to build FIRST |

## Red Flags -- STOP and Reconsider

- You are about to run a command that modifies a file
- You are about to run git commit or git push
- Your recommendation would apply equally well to any random product
- You cannot cite a specific research finding or code observation backing a recommendation
- You are listing more than 10 features (focus beats breadth)
