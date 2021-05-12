// CredHub credential types for generating credentials
package generate

type Password struct {
	Length         int  `json:"length,omitempty"`
	IncludeSpecial bool `json:"include_special,omitempty"`
	ExcludeNumber  bool `json:"exclude_number,omitempty"`
	ExcludeUpper   bool `json:"exclude_upper,omitempty"`
	ExcludeLower   bool `json:"exclude_lower,omitempty"`
}

type User struct {
	Username       string `json:"-"`
	Length         int    `json:"length,omitempty"`
	IncludeSpecial bool   `json:"include_special,omitempty"`
	ExcludeNumber  bool   `json:"exclude_number,omitempty"`
	ExcludeUpper   bool   `json:"exclude_upper,omitempty"`
	ExcludeLower   bool   `json:"exclude_lower,omitempty"`
}

type Certificate struct {
	KeyLength        int      `json:"key_length,omitempty"`
	Duration         int      `json:"duration,omitempty"`
	CommonName       string   `json:"common_name,omitempty"`
	Organization     string   `json:"organization,omitempty"`
	OrganizationUnit string   `json:"organization_unit,omitempty"`
	Locality         string   `json:"locality,omitempty"`
	State            string   `json:"state,omitempty"`
	Country          string   `json:"country,omitempty"`
	AlternativeNames []string `json:"alternative_names,omitempty"`
	KeyUsage         []string `json:"key_usage,omitempty"`
	ExtendedKeyUsage []string `json:"extended_key_usage,omitempty"`
	Ca               string   `json:"ca"`
	IsCA             bool     `json:"is_ca,omitempty"`
	SelfSign         bool     `json:"self_sign,omitempty"`
}

type RSA struct {
	KeyLength int `json:"key_length"`
}

type SSH struct {
	Comment   string `json:"ssh_comment,omitempty"`
	KeyLength int    `json:"key_length"`
}
