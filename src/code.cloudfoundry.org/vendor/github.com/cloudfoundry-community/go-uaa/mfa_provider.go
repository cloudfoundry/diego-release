package uaa

// MFAProvidersEndpoint is the path to the MFA providers resource.
const MFAProvidersEndpoint string = "/mfa-providers"

// MFAProviderConfig is configuration for an MFA provider
type MFAProviderConfig struct {
	Issuer              string `json:"issuer,omitempty"`
	ProviderDescription string `json:"providerDescription,omitempty"`
}

// MFAProvider is a UAA MFA provider
// http://docs.cloudfoundry.org/api/uaa/version/4.19.0/index.html#get-2
type MFAProvider struct {
	ID             string            `json:"id,omitempty"`
	Name           string            `json:"name"`
	IdentityZoneID string            `json:"identityZoneId,omitempty"`
	Config         MFAProviderConfig `json:"config"`
	Type           string            `json:"type"`
	Created        int               `json:"created,omitempty"`
	LastModified   int               `json:"last_modified,omitempty"`
}

// Identifier returns the field used to uniquely identify a MFAProvider.
func (m MFAProvider) Identifier() string {
	return m.ID
}
