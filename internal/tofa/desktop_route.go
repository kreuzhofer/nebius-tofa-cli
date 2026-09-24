package tofa

// Only served by a live launch's authenticated loopback adapter. It is never
// written to the durable bridge or desktop settings.
type desktopRoute struct {
	Bridge    string
	Engine    string
	Home      string
	Overrides []string
}
