package main

import (
	"log"
	"time"
)

const dateLayout = "2006-01-02"

// GenerateDueTransactions walks every active recurring rule and creates a
// real Transaction for every occurrence that's due (today or earlier),
// advancing NextDue as it goes. Capped per-rule so a rule that's been
// inactive for years can't spin off thousands of catch-up transactions in
// one go — anything beyond the cap just resumes from where it left off next
// time this runs.
func GenerateDueTransactions(txStore TransactionStore, recStore RecurringStore) {
	today := time.Now().Format(dateLayout)
	const maxCatchUpPerRule = 60

	// ListAll (not List) deliberately — this background sweep has to
	// cross every tenant's rules, not just one request's. Each
	// generated transaction is created into the rule's own tenant
	// (rule.UserID), so occurrences never leak across accounts.
	for _, rule := range recStore.ListAll() {
		if !rule.Active || rule.NextDue == "" {
			continue
		}

		due := rule.NextDue
		created := 0

		for due <= today && created < maxCatchUpPerRule {
			tx := &Transaction{
				Type:        rule.Type,
				Amount:      rule.Amount,
				Category:    rule.Category,
				Description: rule.Description,
				Date:        due,
			}
			if err := txStore.Create(rule.UserID, tx); err != nil {
				log.Printf("recurring: failed to create transaction for rule %s: %v", rule.ID, err)
				break
			}
			due = advanceDate(due, rule.Frequency)
			created++
		}

		if created > 0 {
			rule.NextDue = due
			if _, err := recStore.Update(rule.UserID, rule.ID, rule); err != nil {
				log.Printf("recurring: failed to update next_due for rule %s: %v", rule.ID, err)
			} else {
				log.Printf("recurring: generated %d occurrence(s) for %q, next due %s", created, rule.Description, due)
			}
		}
	}
}

func advanceDate(dateStr, frequency string) string {
	t, err := time.Parse(dateLayout, dateStr)
	if err != nil {
		// Malformed date shouldn't happen via the API's own validation, but
		// bail out safely rather than looping forever.
		return dateStr
	}
	switch frequency {
	case "weekly":
		t = t.AddDate(0, 0, 7)
	default: // "monthly" and any unrecognized value default to monthly
		t = t.AddDate(0, 1, 0)
	}
	return t.Format(dateLayout)
}
