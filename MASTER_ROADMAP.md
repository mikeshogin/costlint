# Master Roadmap - Linter Ecosystem

Overview of all repositories and their progress.

## archlint (mshogin/archlint)

Architecture analysis and compliance tool.

### Done
- [x] fix: double split in metrics.go (issue #9)
- [x] fix: resource leak in server.go (issue #8)

### In Progress
- [ ] Rust rewrite - core graph engine (issues #17, #18, #19)
- [ ] GitHub Actions CI pipeline (issue #11)
- [ ] DIP refactoring - StateReader/MetricsProvider (issue #10)
- [ ] Tarjan's SCC algorithm for cycle detection (issue #5)

### Planned
- [ ] Auto-detect project language (issue #29)
- [ ] SOLID score per component (issue #30)
- [ ] PR architecture review (issue #31)
- [ ] Architecture health badge (issue #24)
- [ ] GitHub Action in Marketplace (issue #44)
- [ ] Weekly digest to Telegram (issue #43)

---

## promptlint (mikeshogin/promptlint)

Prompt analysis and model routing.

### Done
- [x] Go project setup (issue #6)
- [x] Basic regex metrics (issue #2)
- [x] Domain classification (issue #4)
- [x] Telemetry collection (issue #7)
- [x] Complexity scoring fix (issue #13)
- [x] ccproxy plugin (issue #14)

### In Progress
- [ ] NLP metrics - POS, NER, complexity (issue #3)
- [ ] Routing engine (issue #5)
- [ ] HTTP API (issue #9)

### Planned
- [ ] Batch analysis endpoint (issue #11)
- [ ] Model tier config (issue #12)
- [ ] Performance metrics (issue #16)

---

## costlint (mikeshogin/costlint)

Cost tracking and optimization for LLM usage.

### In Progress
- [ ] Tiktoken-accurate token counting (issue #1)

### Planned
- [ ] A/B test framework (issue #2)
- [ ] Subscription cost model (issue #3)
- [ ] Promptlint telemetry integration (issue #4)
- [ ] Cache metrics (issue #5)
- [ ] Performance metrics (issue #6)
- [ ] Cost prediction before execution (issue #7)

---

## seclint (mikeshogin/seclint)

Security content policy enforcement.

### In Progress
- [ ] Configurable content policies - .seclint.yaml (issue #1)

### Planned
- [ ] Improved keyword detection with context awareness
- [ ] CI/CD pipeline integration

---

## geniearchi (mikeshogin/geniearchi)

Architecture visualization and collaboration.

### In Progress
- [ ] Docker container with deskd runtime (issue #1)

### Planned
- [ ] Desktop app integration
- [ ] Multi-user collaboration

---

## Cross-Cutting Themes

### Ecosystem Integration
- archlint + costlint: escalation cost tracking (archlint #15)
- archlint + promptlint: prompt telemetry for routing (archlint #13)
- costlint + promptlint: telemetry pipeline (costlint #4)
- Plugin architecture for shared features (archlint #35)
- Architecture changelog - unified format (archlint #34)

### Rust Migration
- archlint Rust rewrite (archlint #17, #18, #19)
- Rust monolith plan - all linters + orchestrator (archlint #19)

### Quality Gates
- myhome integration for archlint (archlint #14)
- myhome integration for promptlint (promptlint #11)

### Observability
- Performance metrics across all tools (archlint #22, promptlint #16, costlint #6)
- Architecture anomaly detection (archlint #41)
- Cost prediction (costlint #7)
