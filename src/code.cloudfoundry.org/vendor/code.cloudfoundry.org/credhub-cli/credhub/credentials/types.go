// CredHub credential types
package credentials

import (
	"encoding/json"

	"code.cloudfoundry.org/credhub-cli/credhub/credentials/values"
	"code.cloudfoundry.org/credhub-cli/errors"
)

// Base fields of a credential
type Base struct {
	Id               string   `json:"id" yaml:"id"`
	Name             string   `json:"name" yaml:"name"`
	Type             string   `json:"type" yaml:"type"`
	Metadata         Metadata `json:"metadata" yaml:"metadata"`
	VersionCreatedAt string   `json:"version_created_at" yaml:"version_created_at"`
}

// Arbitrary metadata for credentials
type Metadata map[string]interface{}

// A generic credential
//
// Used when the Type of the credential is not known ahead of time.
//
// Value will be as unmarshalled by https://golang.org/pkg/encoding/json/#Unmarshal
type Credential struct {
	Base  `yaml:",inline"`
	Value interface{} `json:"value"`
}

func (c Credential) MarshalYAML() (interface{}, error) {
	return c.convertToOutput()
}

func (c Credential) MarshalJSON() ([]byte, error) {
	result, err := c.convertToOutput()
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func (c Credential) convertToOutput() (interface{}, error) {
	result := struct {
		Id               string      `json:"id" yaml:"id"`
		Name             string      `json:"name" yaml:"name"`
		Type             string      `json:"type" yaml:"type"`
		Value            interface{} `json:"value"`
		Metadata         Metadata    `json:"metadata" yaml:"metadata,omitempty"`
		VersionCreatedAt string      `json:"version_created_at" yaml:"version_created_at"`
	}{
		Id:               c.Id,
		Name:             c.Name,
		Type:             c.Type,
		Metadata:         c.Metadata,
		VersionCreatedAt: c.VersionCreatedAt,
	}

	_, ok := c.Value.(string)
	if ok {
		result.Value = c.Value
	} else {
		value, ok := c.Value.(interface{})
		if !ok {
			return nil, errors.NewCatchAllError()
		}
		result.Value = value
	}
	return result, nil
}

// A Value type credential
type Value struct {
	Base  `yaml:",inline"`
	Value values.Value `json:"value"`
}

// A JSON type credential
type JSON struct {
	Base  `yaml:",inline"`
	Value values.JSON `json:"value"`
}

// A Password type credential
type Password struct {
	Base  `yaml:",inline"`
	Value values.Password `json:"value"`
}

// A User type credential
type User struct {
	Base  `yaml:",inline"`
	Value struct {
		values.User  `yaml:",inline"`
		PasswordHash string `json:"password_hash" yaml:"password_hash"`
	} `json:"value"`
}

// A Certificate type credential
type Certificate struct {
	Base  `yaml:",inline"`
	Value values.Certificate `json:"value"`
}

// An RSA type credential
type RSA struct {
	Base  `yaml:",inline"`
	Value values.RSA `json:"value"`
}

// An SSH type credential
type SSH struct {
	Base  `yaml:",inline"`
	Value struct {
		values.SSH           `yaml:",inline"`
		PublicKeyFingerprint string `json:"public_key_fingerprint" yaml:"public_key_fingerprint"`
	} `json:"value"`
}

// Type needed for Bulk Regenerate functionality
type BulkRegenerateResults struct {
	Certificates []string `json:"regenerated_credentials" yaml:"regenerated_credentials"`
}

// Types needed for Find functionality
type FindResults struct {
	Credentials []struct {
		Name             string `json:"name" yaml:"name"`
		VersionCreatedAt string `json:"version_created_at" yaml:"version_created_at"`
	} `json:"credentials" yaml:"credentials"`
}

type Paths struct {
	Paths []Path `json:"paths" yaml:"paths"`
}

type Path struct {
	Path string `json:"path" yaml:"path"`
}

type CertificateMetadata struct {
	Id       string                       `json:"id" yaml:"id"`
	Name     string                       `json:"name" yaml:"name"`
	SignedBy string                       `json:"signed_by" yaml:"signed_by"`
	Signs    []string                     `json:"signs" yaml:"signs"`
	Versions []CertificateMetadataVersion `json:"versions" yaml:"versions"`
}

type CertificateMetadataVersion struct {
	Id                   string `json:"id" yaml:"id"`
	ExpiryDate           string `json:"expiry_date" yaml:"expiry_date"`
	Transitional         bool   `json:"transitional" yaml:"transitional"`
	CertificateAuthority bool   `json:"certificate_authority" yaml:"certificate_authority"`
	SelfSigned           bool   `json:"self_signed" yaml:"self_signed"`
}
