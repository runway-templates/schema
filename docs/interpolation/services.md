# `services.<service>.outputs.<KEY>`

The named output of another service in the same stack. Resolved at render
time, after the referenced service's own env and outputs are rendered.

- The referenced service and output key must exist. If not, validation fails.
- Self-references are not allowed. A service reads its own values via `env.*`.
- References form a dependency graph across services. Cycles are a validation
  error. Runway renders and deploys services in dependency order.