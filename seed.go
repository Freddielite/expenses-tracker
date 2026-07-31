package main

// defaultCategories seeds the category store on first run, so a fresh
// install behaves the same as the old hardcoded frontend lists — but from
// here on, categories are fully user-editable.
func defaultCategories() []*Category {
	return []*Category{
		{Name: "Food", Type: TypeExpense, Color: "#b54834", Icon: "utensils"},
		{Name: "Transport", Type: TypeExpense, Color: "#c9862c", Icon: "car"},
		{Name: "Rent", Type: TypeExpense, Color: "#7a5c1f", Icon: "home"},
		{Name: "Utilities", Type: TypeExpense, Color: "#c9a227", Icon: "zap"},
		{Name: "Data & Airtime", Type: TypeExpense, Color: "#3d7a9e", Icon: "smartphone"},
		{Name: "Health", Type: TypeExpense, Color: "#a13d5c", Icon: "heart-pulse"},
		{Name: "Entertainment", Type: TypeExpense, Color: "#6b4ba1", Icon: "clapperboard"},
		{Name: "Shopping", Type: TypeExpense, Color: "#a15c3d", Icon: "shopping-bag"},
		{Name: "Business", Type: TypeExpense, Color: "#3d5aa1", Icon: "briefcase"},
		{Name: "Other", Type: TypeExpense, Color: "#6b6f76", Icon: "more-horizontal"},
		{Name: "Salary", Type: TypeIncome, Color: "#1f7a5c", Icon: "wallet"},
		{Name: "Freelance", Type: TypeIncome, Color: "#1f8a6a", Icon: "circle-dollar-sign"},
		{Name: "Business", Type: TypeIncome, Color: "#3d5aa1", Icon: "briefcase"},
		{Name: "Gift", Type: TypeIncome, Color: "#2f9e7a", Icon: "gift"},
		{Name: "Other", Type: TypeIncome, Color: "#6b6f76", Icon: "more-horizontal"},
	}
}
