# Domain docs

This repository uses a single-context layout:

- CONTEXT.md at the repository root contains domain terminology.
- docs/adr/ contains architecture decision records.

## Before exploring

Read CONTEXT.md and ADRs relevant to the area being explored.

If these files do not exist, proceed silently. The domain-modeling skill
creates them lazily as terminology and decisions are resolved.

## Use the glossary's vocabulary

Use terms from CONTEXT.md when naming domain concepts in issues,
proposals, hypotheses, and tests.

If a needed concept is missing, reconsider whether it belongs in the
project's vocabulary or note the gap for domain-modeling.

## Flag ADR conflicts

Explicitly identify any existing ADR that a proposal contradicts,
and explain why the decision should be reconsidered.
