# Desktop conversations select their own main model

For [#55](https://github.com/kreuzhofer/tofa-launcher/issues/55), the launcher main
is the initial/default model. Every available Token Factory model with compatible
metadata is enabled in the desktop picker; unqualified combinations are labelled
Experimental without additional opt-in. A conversation keeps its recorded main
and provider across launches, and explicit in-conversation selection changes its
main for subsequent turns without changing its identity, history or provider.
This supersedes ADR 0001's original-main relaunch restriction and the desktop
experimental opt-in rule; CLI experimental selection retains its existing policy.

The current project's eligible catalog bounds ordinary chat routing. The
configured launch Guardian applies to every Token Factory main, including when
that model is also selectable for chat; recognized review requests remain
restricted to that Guardian and native approval enforcement. The catalog is a
launch snapshot: unavailable or metadata-incompatible choices fail explicitly,
with no substitution or automatic provider migration.

Desktop default-model and reasoning selections must not modify concurrent CLI
sessions. The desktop bridge contains the pinned model-default write contract
and returns an explicit session override; the renderer keeps its selected model
locally. Unrelated configuration writes remain native. Restoring a shared settings
snapshot on shutdown would both leak during the session and overwrite concurrent
changes, so it is not used.
