package uaa

// SortOrder defines the sort order when listing users or groups.
type SortOrder string

const (
	// SortAscending sorts in ascending order.
	SortAscending = SortOrder("ascending")
	// SortDescending sorts in descending order.
	SortDescending = SortOrder("descending")
)
