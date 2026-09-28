# Preserve conversation model identity across launches

For the initial selection flow in [#54](https://github.com/kreuzhofer/nebius-tofa-cli/issues/54),
an existing desktop conversation retains its recorded main model and provider;
continuing a Token Factory conversation whose main differs from the launch
selection requires relaunching with its original main model. The effective
Guardian, selected by default or an explicit override, applies to Token Factory
conversations for that launch and must be displayed clearly. This preserves
conversation identity and makes reviewer selection explicit while deliberate
changes to an existing conversation's
main model are deferred to [#55](https://github.com/kreuzhofer/nebius-tofa-cli/issues/55).
