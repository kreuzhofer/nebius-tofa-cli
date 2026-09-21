# Issue tracker: GitHub

Issues and specs live in GitHub Issues for kreuzhofer/nebius-tofa-cli.
Use the `gh` CLI from this clone; it infers the repository from the remote.

## Conventions

- Create: `gh issue create --title "..." --body-file <path>`
- Read: `gh issue view <number> --comments`
- List: `gh issue list --state open --json number,title,body,labels,comments`
  Add label and state filters as needed.
- Comment: `gh issue comment <number> --body-file <path>`
- Add labels: `gh issue edit <number> --add-label "<label>"`
- Remove labels: `gh issue edit <number> --remove-label "<label>"`
- Close: `gh issue close <number> --comment "<reason>"`

For multiline bodies, write the exact text to a temporary file and pass
`--body-file`.

“Publish to the issue tracker” means create a GitHub issue.
“Fetch the relevant ticket” means read the issue and its comments.

## Pull requests as a triage surface

**PRs as a request surface: no.**

## Wayfinding operations

- Map: one issue labelled `wayfinder:map`, containing Notes,
  Decisions-so-far, and Fog.
- Child tickets: link them as GitHub sub-issues. If unavailable, use
  a task list in the map and put `Part of #<map>` in each child.
  Label children `wayfinder:<type>`, where type is research, prototype,
  grilling, or task.
- Blocking: use native GitHub issue dependencies. If unavailable,
  put `Blocked by: #<number>` in the child. All blockers must be closed
  before the child is unblocked.
- Frontier: choose the first open, unassigned, unblocked child in map order.
- Claim: assign the ticket to the driving developer.
- Resolve: comment with the answer, close the child, and append a brief
  summary and link to the map's Decisions-so-far.
