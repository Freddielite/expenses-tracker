package main

import "sort"

// BuildReport aggregates a list of transactions into category and monthly summaries.
func BuildReport(transactions []*Transaction) ReportResponse {
	categoryTotals := make(map[string]*CategorySummary)
	monthTotals := make(map[string]*MonthlySummary)

	var totalIncome, totalExpense float64

	for _, t := range transactions {
		// Category summary (expenses only makes the most sense for "where did my money go",
		// but we include income categories too so the breakdown stays complete).
		cs, ok := categoryTotals[t.Category]
		if !ok {
			cs = &CategorySummary{Category: t.Category}
			categoryTotals[t.Category] = cs
		}
		cs.Total += signedAmount(t)
		cs.Count++

		// Monthly summary
		month := t.Date
		if len(month) >= 7 {
			month = month[:7] // "2026-07-14" -> "2026-07"
		}
		ms, ok := monthTotals[month]
		if !ok {
			ms = &MonthlySummary{Month: month}
			monthTotals[month] = ms
		}

		switch t.Type {
		case TypeIncome:
			totalIncome += t.Amount
			ms.TotalIncome += t.Amount
		case TypeExpense:
			totalExpense += t.Amount
			ms.TotalExpense += t.Amount
		}
		ms.Net = ms.TotalIncome - ms.TotalExpense
	}

	categories := make([]CategorySummary, 0, len(categoryTotals))
	for _, cs := range categoryTotals {
		categories = append(categories, *cs)
	}
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Total < categories[j].Total // most spend (most negative) first
	})

	months := make([]MonthlySummary, 0, len(monthTotals))
	for _, ms := range monthTotals {
		months = append(months, *ms)
	}
	sort.Slice(months, func(i, j int) bool { return months[i].Month < months[j].Month })

	return ReportResponse{
		TotalIncome:      totalIncome,
		TotalExpense:     totalExpense,
		Net:              totalIncome - totalExpense,
		ByCategory:       categories,
		ByMonth:          months,
		TransactionCount: len(transactions),
	}
}

// signedAmount returns expenses as negative numbers so category totals show
// net effect (useful if a category has both income and expense entries).
func signedAmount(t *Transaction) float64 {
	if t.Type == TypeExpense {
		return -t.Amount
	}
	return t.Amount
}
