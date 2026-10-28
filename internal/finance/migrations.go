package finance

import "errors"

const latestFinanceSchema = 2

// Apphost's step runner rejects any lower version as a downgrade. When applying
// our ordered migration list on restart, skip already-applied steps only after
// checking the database does not belong to a future Finance executable.
func (a *App) migrate(version int, schema string) error {
	var current int
	if e := a.h.DB.QueryRow("SELECT coalesce(max(version),0) FROM host_migrations").Scan(&current); e != nil {
		return e
	}
	if current > latestFinanceSchema {
		return errors.New("Finance data needs a newer executable; restore a compatible backup to downgrade")
	}
	if current >= version {
		return nil
	}
	return a.h.Migrate(version, schema)
}
