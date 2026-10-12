package postgresservice_test

import (
	"context"
	"fmt"

	"github.com/faustbrian/go-postgres/v2"
	postgresservice "github.com/faustbrian/go-postgres/v2/adapters/service"
)

var _ postgresservice.Resource = (*postgres.Pool)(nil)

// ExampleNew demonstrates ownership and explicit readiness with an in-memory
// Resource, not a connectivity probe against a PostgreSQL server.
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
