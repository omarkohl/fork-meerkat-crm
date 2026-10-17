package services

import (
	"encoding/json"
	"testing"
	"time"

	"meerkat/monica"
)

// FuzzMonicaMapping decodes arbitrary JSON the way the Monica client does (a hostile or
// buggy Monica server controls it) and runs every mapper on whatever decodes. Nothing may
// panic.
//
// Run: go test ./services -run '^$' -fuzz FuzzMonicaMapping -fuzztime 30s
func FuzzMonicaMapping(f *testing.F) {
	for _, s := range []string{
		`{"id":1,"first_name":"Ada","last_name":"Lovelace","nickname":"","gender":"Woman","is_starred":true,"information":{"dates":{"birthdate":{"date":"1815-12-10T00:00:00Z","is_age_based":false,"is_year_unknown":false}},"career":{"job":"Analyst","company":"X"},"avatar":{"url":"https://monica.example.com/a.jpg","source":"photo"}},"tags":[{"name":"friends"}],"addresses":[{"name":"Home","street":"1","city":"L","country":{"name":"UK"}}],"contactFields":[{"content":"a@b.c","contact_field_type":{"name":"Email","protocol":"mailto:","type":"email"}}]}`,
		`{"information":{"dates":{"birthdate":{"date":"2000-01-01","is_age_based":true,"is_year_unknown":true}}}}`,
		`{"id":2,"contacts":[{"id":1,"first_name":"A"}],"attendees":{"contacts":[{"id":1}]},"summary":"s","happened_at":"2020-02-29","remind_at":"2020-02-29T10:00:00Z","next_expected_date":"2020-02-29T10:00:00Z","frequency_type":"year","frequency_number":0,"title":"t","body":"b","amount":"3.5","name":"n","completed":true,"contact":{"id":1}}`,
		`{"initial_date":"2024-12-31","frequency_type":"month","frequency_number":-1,"title":"x"}`,
		`{"id":3,"relationship_type":{"name":"parent"},"contact_is":{"id":2,"first_name":"B"},"of_contact":{"id":1}}`,
		`[]`, `null`, `{}`,
	} {
		f.Add([]byte(s))
	}

	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	f.Fuzz(func(t *testing.T, data []byte) {
		var c monica.Contact
		if json.Unmarshal(data, &c) == nil {
			MapMonicaContact(c, time.Now())
			MapMonicaRelationship(monica.Relationship{}, &c)
		}
		var a monica.Activity
		if json.Unmarshal(data, &a) == nil {
			MapMonicaActivity(a, "en")
		}
		var n monica.Note
		if json.Unmarshal(data, &n) == nil {
			MapMonicaNote(n, time.Now())
		}
		var r monica.Reminder
		if json.Unmarshal(data, &r) == nil {
			MapMonicaReminder(r, now, "")
			MapMonicaReminder(r, now, "--02-29")
		}
		var rel monica.Relationship
		if json.Unmarshal(data, &rel) == nil {
			MapMonicaRelationship(rel, nil)
			MapMonicaRelationship(rel, &c)
		}
		var call monica.Call
		if json.Unmarshal(data, &call) == nil {
			MapMonicaCall(call, "de", time.Now())
		}
		var task monica.Task
		if json.Unmarshal(data, &task) == nil {
			MapMonicaTask(task, "en", time.Now())
		}
		var gift monica.Gift
		if json.Unmarshal(data, &gift) == nil {
			MapMonicaGift(gift, "en", time.Now())
		}
		var debt monica.Debt
		if json.Unmarshal(data, &debt) == nil {
			MapMonicaDebt(debt, "en", time.Now())
		}
		var snap monica.Snapshot
		if json.Unmarshal(data, &snap) == nil {
			birthdayMonthDayByMonicaID(&snap)
		}
	})
}
