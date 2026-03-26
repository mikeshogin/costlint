# costlint

Token cost tracking and optimization for AI agent workflows. Track spending, run A/B tests, identify savings.

## API

### CLI (pipe-friendly)

```bash
# Count: tokens in text
echo "Fix the bug in server.go" | costlint count

# Estimate: cost for a model
echo "Fix the bug in server.go" | costlint estimate --model opus

# Compare: costs across models
costlint compare --file prompts.jsonl --models haiku,sonnet,opus

# Report: aggregate telemetry
costlint report --period 7d --source telemetry.jsonl
```

### HTTP

```bash
# Start server
costlint serve 8092

# POST /count - count tokens
curl -X POST http://localhost:8092/count \
  -H "Content-Type: application/json" \
  -d '{"text":"Design a payment gateway"}'

# POST /estimate - estimate cost
curl -X POST http://localhost:8092/estimate \
  -H "Content-Type: application/json" \
  -d '{"text":"...", "model":"opus"}'

# GET /health - server status
curl http://localhost:8092/health
```

## Output Format

### Count Response
```json
{
  "text": "Fix the bug in server.go",
  "tokens": 42,
  "chars": 25
}
```

### Estimate Response
```json
{
  "text": "Fix the bug in server.go",
  "model": "opus",
  "tokens": 42,
  "estimated_cost_usd": 0.00126,
  "cost_per_1m_tokens": 15.00
}
```

### Report Response
```json
{
  "period": "7d",
  "total_requests": 342,
  "total_tokens": 1245000,
  "by_model": {
    "opus": {"requests": 48, "tokens": 420000, "cost": 12.60},
    "sonnet": {"requests": 195, "tokens": 650000, "cost": 6.50},
    "haiku": {"requests": 99, "tokens": 175000, "cost": 0.44}
  },
  "estimated_total": 19.54,
  "with_optimal_routing": 8.20,
  "savings_percent": 58,
  "expensive_patterns": [
    {"pattern": "simple fixes routed to opus", "count": 23, "waste": 5.80}
  ]
}
```

## Integration

### With promptlint (routing feedback)

```
promptlint suggests model -> agent executes -> costlint tracks cost -> feedback to promptlint
```

### With archlint (escalation tracking)

```
archlint rejects output -> escalate to expensive model -> costlint tracks escalation costs
```

### With orchestrator (budget enforcement)

```
orchestrator schedules tasks -> costlint provides cost estimates -> enforce budget limits
```

## Install

```bash
go install github.com/mikeshogin/costlint/cmd/costlint@latest
```

## Capabilities

- **Token counting** - count input/output tokens without LLM API calls (tiktoken-compatible)
- **Cost estimation** - map tokens to real costs (subscription tiers, API pricing)
- **A/B testing** - split traffic, collect metrics, compare cost vs quality
- **Cost reports** - breakdown by model, agent, time period
- **Optimization hints** - identify expensive patterns that could use cheaper models

## Pricing Tables

Supports all major model providers:

- Anthropic: haiku, sonnet, opus (input/output rates)
- OpenAI: gpt-3.5, gpt-4, gpt-4-turbo
- Custom: user-defined rates

## Ecosystem

Part of the AI agent cost optimization ecosystem:

- **[promptlint](https://github.com/mikeshogin/promptlint)** - prompt routing by complexity
- **[seclint](https://github.com/mikeshogin/seclint)** - security/content classification
- **[archlint](https://github.com/mshogin/archlint)** - code quality validation

```
prompt -> promptlint (route) -> agent (execute) -> archlint (validate) -> costlint (track cost)
```

See [ECOSYSTEM.md](ECOSYSTEM.md) for full integration.

## For Humans

Costlint helps you understand and optimize what you're spending on AI APIs. It counts tokens in your requests and estimates costs based on current pricing, lets you compare what different models would cost for the same task, and shows you patterns of wasteful spending (like routing simple tasks to expensive models). It integrates with the other tools in the ecosystem to create feedback loops that automatically improve your cost efficiency over time.
