package models

import "net/url"

type BBSPresence struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

func NewBBSPresence(id, url string) BBSPresence {
	return BBSPresence{
		ID:  id,
		URL: url,
	}
}

func (p BBSPresence) Validate() error {
	var validationError ValidationError

	if p.ID == "" {
		validationError = validationError.Append(ErrInvalidField{Field: "id"})
	}

	if p.URL == "" {
		validationError = validationError.Append(ErrInvalidField{Field: "url"})
	}

	url, err := url.Parse(p.URL)
	if err != nil || !url.IsAbs() {
		validationError = validationError.Append(ErrInvalidField{Field: "url"})
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}
