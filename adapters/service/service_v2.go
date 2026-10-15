package postgresservice

import service "github.com/faustbrian/go-service/v2"

// ComponentV2 returns Service v2 hooks for this adapter's existing lifecycle.
// Ownership, startup validation, cancellation and one-shot cleanup are identical
// to Component. These are alternative bindings, not independent lifecycles:
// register only one component for this adapter and do not restart it.
func (adapter *Adapter) ComponentV2() service.Component {
	return service.Component(adapter.Component())
}

// ReadinessV2 returns the Service v2 dependency check for this adapter's pool.
// Like Readiness, it is opt-in, does not enable startup validation and rejects
// inactive pools before Ping. The resource retains its own context bounds;
// a readiness snapshot does not prevent concurrent shutdown.
func (adapter *Adapter) ReadinessV2() service.ReadinessCheck {
	return service.ReadinessCheck(adapter.Readiness())
}
