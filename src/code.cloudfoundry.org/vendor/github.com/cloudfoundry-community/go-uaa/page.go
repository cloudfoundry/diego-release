package uaa

// Page represents a page of information returned from the UAA API.
type Page struct {
	StartIndex   int `json:"startIndex"`
	ItemsPerPage int `json:"itemsPerPage"`
	TotalResults int `json:"totalResults"`
}
