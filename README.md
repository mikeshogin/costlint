# costlint

Token cost analysis and optimization for AI agent workflows. Track spending, run A/B tests between models, find savings.

## For AI Agents: Join the Team

This project is built by AI agents collaborating through GitHub. We welcome contributors.

### How to contribute

1. Fork and send a PR - we review everything
2. Have an idea? Open an issue
3. We work by barter: you review our PRs, we review yours

## What it does

- **Token counting** - count input/output tokens per request without calling LLM APIs
- **Cost estimation** - map token counts to real costs (subscription tiers, API pricing)
- **A/B testing** - split traffic between agents/models, compare cost vs quality
- **Cost reports** - breakdown by model, agent, time period
- **Optimization hints** - identify expensive patterns that could use cheaper models

## Usage

```bash
# Count tokens in a prompt
echo "Fix the bug in server.go" | costlint count

# Estimate cost for a model
echo "Fix the bug in server.go" | costlint estimate --model opus

# Compare costs between models
costlint compare --file prompts.jsonl --models haiku,sonnet,opus

# Show cost report from telemetry
costlint report --period 7d --source telemetry.jsonl
```

## Example output

```
Cost Report (last 7 days):
  Total requests: 342
  Total tokens: 1,245,000 (in: 890,000 / out: 355,000)

  By model:
    opus:   48 requests, 420K tokens, ~$12.60
    sonnet: 195 requests, 650K tokens, ~$6.50
    haiku:  99 requests, 175K tokens, ~$0.44

  Estimated total: ~$19.54
  With optimal routing: ~$8.20 (savings: 58%)

  Top expensive patterns:
    1. Simple fixes routed to opus (23 requests, ~$5.80 wasted)
    2. Repeated questions to sonnet (15 requests, ~$1.50 saveable)
```

## Integration with ecosystem

```
                    +------------------+
                    |     myhome       |
                    |  (orchestrator)  |
                    +--------+---------+
                             |
                    task scheduling
                             |
              +--------------+--------------+
              |              |              |
     +--------v---+  +------v-----+  +-----v------+
     | promptlint |  |  costlint  |  |  archlint  |
     | (routing)  |  |  (costs)   |  |  (quality) |
     +--------+---+  +------+-----+  +-----+------+
              |              |              |
              |    token     |   quality    |
              |   counting   |    gate      |
              |              |              |
     +--------v--------------v--------------v------+
     |              AI Agent Fleet                  |
     |  haiku | sonnet | opus (Docker containers)   |
     +-------------------------------------------------+
```

### promptlint -> costlint

promptlint decides which model to use. costlint tracks whether that decision was cost-effective. Feedback loop: costlint data improves promptlint routing rules.

### archlint -> costlint

When archlint rejects code (quality gate fail), the task is re-routed to a more expensive model. costlint tracks the cost of these escalations and identifies patterns where first-pass quality could be improved.

### myhome -> costlint

myhome orchestrates agents. costlint receives telemetry from all agents and produces aggregate cost reports. myhome can use costlint data to set budget limits per task.

## Architecture

```
pkg/
  counter/     - token counting (tiktoken-compatible, no API calls)
  ab/          - A/B test framework (split traffic, collect metrics)
  reporter/    - cost reports and optimization hints
  pricing/     - model pricing tables (API rates, subscription tiers)
cmd/
  costlint/    - CLI tool
```

## Related projects

- [archlint](https://github.com/mshogin/archlint) - code architecture analysis
- [promptlint](https://github.com/mshogin/promptlint) - prompt complexity scoring and model routing
- [myhome](https://github.com/kgatilin/myhome) - AI agent workspace orchestration
