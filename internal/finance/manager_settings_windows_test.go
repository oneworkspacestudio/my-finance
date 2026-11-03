//go:build windows

package finance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

const fictionalManagerProfile = `{"essentialMonthlyMinor":100000,"emergencySavingsMinor":300000,"highCostDebt":"no","horizonYears":10,"risk":"balanced","goals":[{"name":"Fictional garden","targetMinor":100000,"savedMinor":0,"months":10}],"manager":{"enabled":true,"savingsMonthlyMinor":10000,"investmentMonthlyMinor":5000,"cashBufferMinor":1000,"categories":[{"name":"Fictional groceries","essential":true,"targetMonthlyMinor":null}]}}`

func TestManagerSettingsProtectedPerUserAndBudget(t *testing.T) {
	a := aiHostFixture(t)
	scope := strings.Repeat("a", 64)
	input := `{"scope":"` + scope + `","profile":` + fictionalManagerProfile + `}`
	if w := aiFixtureRequest(a, "", "POST", "manager-settings", input, nil); w.Code != 401 {
		t.Fatal("anonymous save accepted", w.Code)
	}
	for name, alter := range map[string]func(*http.Request){
		"csrf":     func(r *http.Request) { r.Header.Del("X-CSRF-Token") },
		"origin":   func(r *http.Request) { r.Header.Set("Origin", "https://invalid.example") },
		"identity": func(r *http.Request) { r.Header.Set("X-Workspace-Expected-Account", "someone-else") },
	} {
		t.Run(name, func(t *testing.T) {
			w := aiFixtureRequest(a, "fixture-owner", "POST", "manager-settings", input, alter)
			if w.Code != 403 && w.Code != 409 {
				t.Fatal("guard bypassed", w.Code)
			}
		})
	}
	w := aiFixtureRequest(a, "fixture-owner", "POST", "manager-settings", input, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"saved":true`) {
		t.Fatal(w.Code, w.Body.String())
	}
	var cipher []byte
	if e := a.h.DB.QueryRow(`SELECT secret FROM finance_manager_settings WHERE user_id=? AND scope=?`, "fixture-owner", scope).Scan(&cipher); e != nil || bytes.Contains(cipher, []byte("Fictional groceries")) || bytes.Contains(cipher, []byte("Fictional garden")) {
		t.Fatal("sensitive rules not protected", e)
	}
	w = aiFixtureRequest(a, "fixture-owner", "GET", "manager-settings?scope="+scope, "", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Fictional garden") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("saved rules unreadable", w.Code)
	}
	for _, pair := range [][2]string{{"fixture-member", scope}, {"fixture-owner", "manual"}} {
		w = aiFixtureRequest(a, pair[0], "GET", "manager-settings?scope="+pair[1], "", nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"configured":false`) {
			t.Fatal("rules crossed user/budget boundary", w.Code)
		}
	}
	// A valid DPAPI blob must also match both authenticated identity and budget scope.
	for _, pair := range [][2]string{{"fixture-member", scope}, {"fixture-owner", "manual"}} {
		if _, e := a.h.DB.Exec(`INSERT INTO finance_manager_settings(user_id,scope,secret)VALUES(?,?,?)`, pair[0], pair[1], cipher); e != nil {
			t.Fatal(e)
		}
		w = aiFixtureRequest(a, pair[0], "GET", "manager-settings?scope="+pair[1], "", nil)
		if w.Code != 422 || strings.Contains(w.Body.String(), "Fictional garden") {
			t.Fatal("ciphertext substitution accepted", w.Code)
		}
	}
	w = aiFixtureRequest(a, "fixture-member", "POST", "manager-settings", `{"scope":"`+scope+`","remove":true}`, nil)
	if w.Code != 200 {
		t.Fatal("remove failed")
	}
	w = aiFixtureRequest(a, "fixture-owner", "GET", "manager-settings?scope="+scope, "", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"configured":true`) {
		t.Fatal("member deleted owner rules")
	}
}

func TestManagerSettingsLargeProfileRoundtrip(t *testing.T) {
	a := aiHostFixture(t)
	var profile aiProfile
	if e := json.Unmarshal([]byte(fictionalManagerProfile), &profile); e != nil {
		t.Fatal(e)
	}
	profile.Manager.Categories = nil
	for i := 0; i < 200; i++ {
		profile.Manager.Categories = append(profile.Manager.Categories, aiCategoryRule{Name: fmt.Sprintf("Fictional category %03d %s", i, strings.Repeat("&", 50)), Essential: true})
	}
	request, e := json.Marshal(managerSettingsInput{Scope: "manual", Profile: &profile})
	if e != nil {
		t.Fatal(e)
	}
	// JSON HTML escaping would needlessly expand this boundary fixture beyond the
	// HTTP input bound. Browsers use literal ampersands, as this test does here.
	request = bytes.ReplaceAll(request, []byte(`\u0026`), []byte("&"))
	if len(request) <= 4096 || len(request) > 64<<10 {
		t.Fatal("fixture size", len(request))
	}
	w := aiFixtureRequest(a, "fixture-owner", "POST", "manager-settings", string(request), nil)
	if w.Code != 200 {
		t.Fatal("large profile save failed", w.Code, w.Body.String())
	}
	w = aiFixtureRequest(a, "fixture-owner", "GET", "manager-settings?scope=manual", "", nil)
	var saved struct {
		Configured bool      `json:"configured"`
		Profile    aiProfile `json:"profile"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &saved) != nil || !saved.Configured || len(saved.Profile.Manager.Categories) != 200 || saved.Profile.Manager.Categories[199].Name != profile.Manager.Categories[199].Name {
		t.Fatal("large profile read failed", w.Code)
	}
	if _, e := protectSession(make([]byte, 4097)); e == nil {
		t.Fatal("session reference limit was weakened")
	}
	if _, e := protectManagerProfile(make([]byte, (128<<10)+1)); e == nil {
		t.Fatal("manager envelope is unbounded")
	}
}

func TestManagerSettingsRejectsInvalidInputAndBoundsStorage(t *testing.T) {
	a := aiHostFixture(t)
	for _, body := range []string{
		`{"scope":"../private","profile":` + fictionalManagerProfile + `}`,
		`{"scope":"manual"}`,
		`{"scope":"manual","profile":{"highCostDebt":"invalid"}}`,
		`{"scope":"manual","profile":` + fictionalManagerProfile + `,"transactions":[]}`,
		`{"scope":"manual","profile":` + fictionalManagerProfile + `} {}`,
		strings.Replace(`{"scope":"manual","profile":`+fictionalManagerProfile+`}`, `"savingsMonthlyMinor":10000`, `"savingsMonthlyMinor":-1`, 1),
	} {
		w := aiFixtureRequest(a, "fixture-owner", "POST", "manager-settings", body, nil)
		if w.Code != 400 {
			t.Fatal("invalid rules accepted", w.Code)
		}
	}
	for i := 0; i < 50; i++ {
		w := aiFixtureRequest(a, "fixture-owner", "POST", "manager-settings", fmt.Sprintf(`{"scope":"%064x","profile":%s}`, i, fictionalManagerProfile), nil)
		if w.Code != 200 {
			t.Fatal("bounded save failed", i, w.Code, w.Body.String())
		}
	}
	w := aiFixtureRequest(a, "fixture-owner", "POST", "manager-settings", `{"scope":"manual","profile":`+fictionalManagerProfile+`}`, nil)
	if w.Code != 422 {
		t.Fatal("unbounded storage", w.Code)
	}
	w = aiFixtureRequest(a, "fixture-owner", "POST", "manager-settings", fmt.Sprintf(`{"scope":"%064x","profile":%s}`, 0, fictionalManagerProfile), nil)
	if w.Code != 200 {
		t.Fatal("existing rule update blocked by limit", w.Code)
	}
}
