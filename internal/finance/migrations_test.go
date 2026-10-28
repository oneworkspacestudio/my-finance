package finance

import (
	"github.com/oneworkspacestudio/one-workspace/pkg/apphost"
	"testing"
)

func TestFinanceMigrationsSurviveRestartAndRefuseNewerSchema(t *testing.T) {
	c := apphost.Config{ID: "finance", Name: "Migration fixture", Version: "fixture", Data: t.TempDir(), Listen: "127.0.0.1:25189", Origin: "http://127.0.0.1:25189/finance"}
	open := func() *App {
		h, e := apphost.New(c, nil)
		if e != nil {
			t.Fatal(e)
		}
		return &App{h: h}
	}
	a := open()
	if e := a.connectionRoutes(); e != nil {
		t.Fatal(e)
	}
	if e := a.snapshotRoutes(); e != nil {
		t.Fatal(e)
	}
	if _, e := a.h.DB.Exec("UPDATE finance_snapshot_schedule SET last_success='fictional-retained-receipt' WHERE id=1"); e != nil {
		t.Fatal(e)
	}
	a.h.Close()
	a = open()
	if e := a.connectionRoutes(); e != nil {
		t.Fatal("restart rejected an already applied migration", e)
	}
	if e := a.snapshotRoutes(); e != nil {
		t.Fatal(e)
	}
	var saved string
	if e := a.h.DB.QueryRow("SELECT last_success FROM finance_snapshot_schedule WHERE id=1").Scan(&saved); e != nil || saved != "fictional-retained-receipt" {
		t.Fatal("restart lost settings", e)
	}
	if e := a.h.Migrate(3, "SELECT 1"); e != nil {
		t.Fatal(e)
	}
	a.h.Close()
	a = open()
	defer a.h.Close()
	if e := a.connectionRoutes(); e == nil {
		t.Fatal("future schema accepted by older executable")
	}
}
