package postgresservice_test

import (
	"context"
	"fmt"

	"github.com/faustbrian/go-postgres/v2"
	"github.com/faustbrian/go-postgres/v2/postgresservice"
)

var _ postgresservice.Resource = (*postgres.Pool)(nil)

// ExampleNew preserves the released adapter path with a fake Resource; new
// consumers should use adapters/service for the same lifecycle contract.
func ExampleNew() {
	pool := &resource{}
	adapter, err := postgresservice.New(postgresservice.Options{
		Name:              "database",
		Pool:              pool,
		TransferOwnership: true,
		StartupPing:       true,
	})
	if err != nil {
		fmt.Println("adapter setup failed")

		return
	}

	component := adapter.Component()
	_ = component.Start(context.Background())
	check := adapter.Readiness()
	_ = check.Run(context.Background())
	_ = component.Stop(context.Background())

	fmt.Println(pool.pings, pool.closes)
	// Output: 2 1
}
