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
	if e := a.aiRoutes(); e != nil {
		t.Fatal("schema 2 to 4 upgrade failed", e)
	}
	a.h.Close()
	a = open()
	if e := a.connectionRoutes(); e != nil {
		t.Fatal(e)
	}
	if e := a.snapshotRoutes(); e != nil {
		t.Fatal(e)
	}
	if e := a.aiRoutes(); e != nil {
		t.Fatal("schema 4 restart failed", e)
	}
	if e := a.investmentRoutes(); e != nil {
		t.Fatal("schema 4 to 5 paper upgrade failed", e)
	}
	if e := a.brokerRoutes(); e != nil {
		t.Fatal("schema 5 to 6 broker upgrade failed", e)
	}
	a.h.Close()
	a = open()
	if e := a.aiRoutes(); e != nil {
		t.Fatal(e)
	}
	if e := a.investmentRoutes(); e != nil {
		t.Fatal(e)
	}
	if e := a.brokerRoutes(); e != nil {
		t.Fatal("schema 6 restart failed", e)
	}
	if e := a.h.DB.QueryRow("SELECT last_success FROM finance_snapshot_schedule WHERE id=1").Scan(&saved); e != nil || saved != "fictional-retained-receipt" {
		t.Fatal("AI migration lost existing snapshot settings", e)
	}
	if e := a.h.Migrate(latestFinanceSchema+1, "SELECT 1"); e != nil {
		t.Fatal(e)
	}
	a.h.Close()
	a = open()
	defer a.h.Close()
	if e := a.connectionRoutes(); e == nil {
		t.Fatal("future schema accepted by older executable")
	}
}

func TestFinance3To4KeepsExistingAIKeyRecords(t *testing.T) {
	c := apphost.Config{ID: "finance", Name: "Fictional upgrade fixture", Version: "fixture", Data: t.TempDir(), Listen: "127.0.0.1:25191", Origin: "http://127.0.0.1:25191/finance"}
	h, e := apphost.New(c, nil)
	if e != nil {
		t.Fatal(e)
	}
	a := &App{h: h}
	if e = a.connectionRoutes(); e != nil {
		t.Fatal(e)
	}
	if e = a.snapshotRoutes(); e != nil {
		t.Fatal(e)
	}
	if e = a.migrate(3, `CREATE TABLE finance_ai_keys(user_id TEXT NOT NULL,provider TEXT NOT NULL,model TEXT NOT NULL,secret BLOB NOT NULL,PRIMARY KEY(user_id,provider));`); e != nil {
		t.Fatal(e)
	}
	if _, e = h.DB.Exec(`INSERT INTO finance_ai_keys VALUES('fictional-owner','openai','fictional-model',x'01020304')`); e != nil {
		t.Fatal(e)
	}
	h.Close()
	h, e = apphost.New(c, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer h.Close()
	a = &App{h: h}
	if e = a.aiRoutes(); e != nil {
		t.Fatal("3 to 4 migration", e)
	}
	var model, secret string
	if e = h.DB.QueryRow(`SELECT model,hex(secret) FROM finance_ai_keys WHERE user_id='fictional-owner'`).Scan(&model, &secret); e != nil || model != "fictional-model" || secret != "01020304" {
		t.Fatal("upgrade changed existing AI credentials", e)
	}
	var version int
	if e = h.DB.QueryRow(`SELECT max(version) FROM host_migrations`).Scan(&version); e != nil || version != 4 {
		t.Fatal("schema 4 was not installed", version, e)
	}
}
