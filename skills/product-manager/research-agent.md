# Research Agent Prompt Template

Use this template to construct prompts for each research subagent dispatched in Phase 2. Fill in the bracketed sections with application-specific details from Phase 1.

---

## Base Prompt

```
You are a product research analyst. Your task is to research one specific dimension
of the product space for an application. Use web search and page fetches to gather
real, current information. Do not speculate -- if you can't find data, say so.

**Application context:**
- **What it does**: [one-paragraph summary from Phase 1]
- **Target user**: [persona identified in Phase 1]
- **Tech stack**: [languages, frameworks, deployment model]
- **Category**: [product category, e.g. "developer CLI tool for database migrations"]

**Your research domain**: [Competitor Analysis | Market Trends | User Pain Points | Distribution Channels]
```

---

## Domain-Specific Instructions

### Competitor Analysis

Research 5-8 products that compete directly or adjacently. For each competitor, report:

1. **Name and URL**
2. **Positioning**: How they describe themselves (use their actual tagline/hero text)
3. **Pricing model**: Free, freemium, paid tiers, open-source, enterprise -- verify by reading their pricing page
4. **Key features**: What they do well
5. **Weaknesses**: What users complain about (search "[competitor] problems" or "[competitor] alternative")
6. **Market signals**: GitHub stars, npm downloads, funding rounds, employee count, press mentions
7. **Differentiation**: What they do that the target application doesn't, and vice versa

Search queries to try:
- "[category] tools 2026"
- "[category] alternatives"
- "best [category] comparison"
- "[specific competitor] vs"
- "[category] open source"

### Market Trends

Research the broader market. Report:

1. **Market size and growth**: Any available TAM/SAM data, growth rates
2. **Recent developments**: Major launches, acquisitions, pivots in the last 12 months
3. **Technology shifts**: New technologies or approaches gaining traction
4. **User expectation shifts**: How expectations are evolving
5. **Consolidation vs. fragmentation**: Few dominant players or many niches?
6. **Regulatory or platform changes**: Policy changes affecting this space

Search queries to try:
- "[category] market 2026"
- "[category] trends"
- "[category] industry report"
- "[adjacent technology] impact on [category]"

### User Pain Points

Research what users in this space struggle with. Report:

1. **Recurring complaints**: Top 5-10 pain points with example quotes or links
2. **Unmet needs**: Features users ask for that no product delivers well
3. **Switching friction**: Why users stay with inferior products
4. **Workarounds**: Hacks or workflows users have built to compensate for gaps
5. **Sentiment signals**: Overall satisfaction level in the space

Search queries to try:
- "[category] frustrations reddit"
- "[category] problems hacker news"
- "[competitor] issues github"
- "[competitor] complaints"
- "why I switched from [competitor]"
- "[category] wishlist"

Open and read specific forum threads that surface in search results.

### Distribution Channels

Research how products in this space reach users. Report:

1. **Primary channels**: Where users discover and install products in this category
2. **Package registries**: Relevant registries and download volumes for competitors
3. **Marketplace presence**: IDE marketplaces, app stores, platform ecosystems
4. **Content marketing patterns**: What content drives adoption in this space
5. **Community dynamics**: Where the community congregates (Discord, Slack, Reddit, forums)
6. **Enterprise vs. self-serve**: Top-down or bottom-up adoption?
7. **Open-source dynamics**: How OSS strategy drives commercial adoption (if applicable)

Search queries to try:
- "how to install [competitor]"
- "[category] getting started"
- "[competitor] growth story"
- "[category] developer marketing"
- "[category] enterprise adoption"

---

## Output Format

Return findings as structured markdown with clear headers. Include:
- Specific names, numbers, and URLs for every claim
- Direct quotes where available
- Confidence level for each finding (high/medium/low based on source quality)
- **Sources** section at the end with all URLs referenced

Every finding must be specific and sourced. If you cannot find data on a topic, say so explicitly rather than filling with speculation.
