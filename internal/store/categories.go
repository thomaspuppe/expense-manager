package store

// Category is one fixed spending category. The set is embedded in the binary
// (KTD3, KTD5): categories are persisted on an expense by their stable Key, so
// renaming a Label or Icon later never rewrites history.
type Category struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Icon  string `json:"icon"`
}

// Categories is the fixed default set for v1. No per-user customization (KD4).
var Categories = []Category{
	{Key: "groceries", Label: "Groceries", Icon: "🛒"},
	{Key: "eating_out", Label: "Eating out", Icon: "🍴"},
	{Key: "transport", Label: "Transport", Icon: "🚇"},
	{Key: "home", Label: "Home", Icon: "🏠"},
	{Key: "health", Label: "Health", Icon: "💊"},
	{Key: "leisure", Label: "Leisure", Icon: "🎉"},
	{Key: "shopping", Label: "Shopping", Icon: "🛍️"},
	{Key: "bills", Label: "Bills", Icon: "🧾"},
	{Key: "travel", Label: "Travel", Icon: "✈️"},
	{Key: "other", Label: "Other", Icon: "📦"},
}

var categoryKeys = func() map[string]bool {
	m := make(map[string]bool, len(Categories))
	for _, c := range Categories {
		m[c.Key] = true
	}
	return m
}()

// IsValidCategory reports whether key names one of the fixed categories.
func IsValidCategory(key string) bool {
	return categoryKeys[key]
}
