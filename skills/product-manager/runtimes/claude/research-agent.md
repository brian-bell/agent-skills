# Research Agent — Workflow Mode Schema Addendum

Domain briefs, search queries, and the standard markdown output live in [roles/researcher.md](roles/researcher.md). This file is Claude-only: use it when Phase 2 runs in **Workflow mode** so each dimension's `agent()` call returns schema-validated JSON instead of markdown.

Claims marked `time_sensitive: true` feed the adversarial verification stage.

## Shared shape

Every dimension returns:

```json
{
  "findings": [
    {
      "claim": "one-sentence factual assertion",
      "detail": "supporting specifics, numbers, quotes",
      "source_url": "primary source URL",
      "confidence": "high | medium | low",
      "time_sensitive": true
    }
  ]
}
```

Set `time_sensitive: true` for anything that decays — pricing, funding, market size, download/star counts, employee counts, recency-dependent positioning. The verification stage refutes-or-downgrades exactly these.

## Per-dimension fields

Added alongside `findings[]`:

- **competitors** — `competitors[]`, each: `{ name, url, positioning, pricing_model, strengths, weaknesses, market_signals }`
- **market-trends** — `trends[]`, each: `{ trend, direction (growing|consolidating|fragmenting), evidence, source_url, time_sensitive }`; plus optional `market_size { tam, growth_rate, source_url }`
- **pain-points** — `pain_points[]`, each: `{ pain, evidence_quote, source_url, frequency (recurring|occasional), addressable_by_product (yes|no|maybe) }`
- **distribution** — `channels[]`, each: `{ channel, mechanism, evidence, adoption_motion (self-serve|top-down|community), source_url }`

Keep the JSON strictly to these fields so the schema validates. Anything you'd normally footnote in prose belongs in `detail` or `source_url`.
