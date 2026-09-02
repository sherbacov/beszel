package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Adds the `addresses` field to system_details. The agent reports the global IP
// addresses of each interface; storing them lets the UI show which addresses a
// machine is actually reachable on, rather than only the single `host` value the
// hub happened to connect to.
func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("system_details")
		if err != nil {
			return err
		}
		if collection.Fields.GetByName("addresses") != nil {
			return nil
		}
		collection.Fields.Add(&core.JSONField{
			Name:    "addresses",
			MaxSize: 100_000,
		})
		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("system_details")
		if err != nil {
			return err
		}
		collection.Fields.RemoveByName("addresses")
		return app.Save(collection)
	})
}
