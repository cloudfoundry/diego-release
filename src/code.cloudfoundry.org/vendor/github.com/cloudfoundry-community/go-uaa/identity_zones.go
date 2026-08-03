package uaa

// IdentityZonesEndpoint is the path to the users resource.
const IdentityZonesEndpoint string = "/identity-zones"

// IdentityZone is a UAA identity zone.
// http://docs.cloudfoundry.org/api/uaa/version/4.14.0/index.html#identity-zones
type IdentityZone struct {
	ID           string             `json:"id,omitempty"`
	Subdomain    string             `json:"subdomain"`
	Config       IdentityZoneConfig `json:"config"`
	Name         string             `json:"name"`
	Version      int                `json:"version,omitempty"`
	Description  string             `json:"description,omitempty"`
	Created      int                `json:"created,omitempty"`
	LastModified int                `json:"last_modified,omitempty"`
}

// Identifier returns the field used to uniquely identify an IdentityZone.
func (iz IdentityZone) Identifier() string {
	return iz.ID
}

// ClientSecretPolicy is an identity zone client secret policy.
type ClientSecretPolicy struct {
	MinLength                 int `json:"minLength,omitempty"`
	MaxLength                 int `json:"maxLength,omitempty"`
	RequireUpperCaseCharacter int `json:"requireUpperCaseCharacter,omitempty"`
	RequireLowerCaseCharacter int `json:"requireLowerCaseCharacter,omitempty"`
	RequireDigit              int `json:"requireDigit,omitempty"`
	RequireSpecialCharacter   int `json:"requireSpecialCharacter,omitempty"`
}

// TokenPolicy is an identity zone token policy.
type TokenPolicy struct {
	AccessTokenValidity  int    `json:"accessTokenValidity,omitempty"`
	RefreshTokenValidity int    `json:"refreshTokenValidity,omitempty"`
	JWTRevocable         bool   `json:"jwtRevocable,omitempty"`
	RefreshTokenUnique   bool   `json:"refreshTokenUnique,omitempty"`
	RefreshTokenFormat   string `json:"refreshTokenFormat,omitempty"`
	ActiveKeyID          string `json:"activeKeyId,omitempty"`
}

// SAMLKey is an identity zone SAML key.
type SAMLKey struct {
	Key         string `json:"key,omitempty"`
	Passphrase  string `json:"passphrase,omitempty"`
	Certificate string `json:"certificate,omitempty"`
}

// SAMLConfig is an identity zone SAMLConfig.
type SAMLConfig struct {
	AssertionSigned            bool               `json:"assertionSigned,omitempty"`
	RequestSigned              bool               `json:"requestSigned,omitempty"`
	WantAssertionSigned        bool               `json:"wantAssertionSigned,omitempty"`
	WantAuthnRequestSigned     bool               `json:"wantAuthnRequestSigned,omitempty"`
	AssertionTimeToLiveSeconds int                `json:"assertionTimeToLiveSeconds,omitempty"`
	ActiveKeyID                string             `json:"activeKeyId,omitempty"`
	Keys                       map[string]SAMLKey `json:"keys,omitempty"`
	DisableInResponseToCheck   bool               `json:"disableInResponseToCheck,omitempty"`
}

// CORSPolicy is an identity zone CORSPolicy.
type CORSPolicy struct {
	XHRConfiguration struct {
		AllowedOrigins        []string      `json:"allowedOrigins,omitempty"`
		AllowedOriginPatterns []interface{} `json:"allowedOriginPatterns,omitempty"`
		AllowedURIs           []string      `json:"allowedUris,omitempty"`
		AllowedURIPatterns    []interface{} `json:"allowedUriPatterns,omitempty"`
		AllowedHeaders        []string      `json:"allowedHeaders,omitempty"`
		AllowedMethods        []string      `json:"allowedMethods,omitempty"`
		AllowedCredentials    bool          `json:"allowedCredentials,omitempty"`
		MaxAge                int           `json:"maxAge,omitempty"`
	} `json:"xhrConfiguration,omitempty"`
	DefaultConfiguration struct {
		AllowedOrigins        []string      `json:"allowedOrigins,omitempty"`
		AllowedOriginPatterns []interface{} `json:"allowedOriginPatterns,omitempty"`
		AllowedURIs           []string      `json:"allowedUris,omitempty"`
		AllowedURIPatterns    []interface{} `json:"allowedUriPatterns,omitempty"`
		AllowedHeaders        []string      `json:"allowedHeaders,omitempty"`
		AllowedMethods        []string      `json:"allowedMethods,omitempty"`
		AllowedCredentials    bool          `json:"allowedCredentials,omitempty"`
		MaxAge                int           `json:"maxAge,omitempty"`
	} `json:"defaultConfiguration,omitempty"`
}

// IdentityZoneLinks is an identity zone link.
type IdentityZoneLinks struct {
	Logout struct {
		RedirectURL              string   `json:"redirectUrl,omitempty"`
		RedirectParameterName    string   `json:"redirectParameterName,omitempty"`
		DisableRedirectParameter bool     `json:"disableRedirectParameter,omitempty"`
		Whitelist                []string `json:"whitelist,omitempty"`
	} `json:"logout,omitempty"`
	HomeRedirect string `json:"homeRedirect,omitempty"`
	SelfService  struct {
		SelfServiceLinksEnabled bool   `json:"selfServiceLinksEnabled,omitempty"`
		Signup                  string `json:"signup,omitempty"`
		Passwd                  string `json:"passwd,omitempty"`
	} `json:"selfService,omitempty"`
}

// Prompt is a UAA prompt.
type Prompt struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

// Branding is the branding for a UAA identity zone.
type Branding struct {
	CompanyName string `json:"companyName,omitempty"`
	ProductLogo string `json:"productLogo,omitempty"`
	SquareLogo  string `json:"squareLogo,omitempty"`
}

// IdentityZoneUserConfig is the user configuration for an identity zone.
type IdentityZoneUserConfig struct {
	DefaultGroups []string `json:"defaultGroups,omitempty"`
}

// IdentityZoneMFAConfig is the MFA configuration for an identity zone.
type IdentityZoneMFAConfig struct {
	Enabled      *bool  `json:"enabled,omitempty"`
	ProviderName string `json:"providerName,omitempty"`
}

// IdentityZoneConfig is the configuration for an identity zone.
type IdentityZoneConfig struct {
	ClientSecretPolicy    *ClientSecretPolicy     `json:"clientSecretPolicy,omitempty"`
	TokenPolicy           *TokenPolicy            `json:"tokenPolicy,omitempty"`
	SAMLConfig            *SAMLConfig             `json:"samlConfig,omitempty"`
	CORSPolicy            *CORSPolicy             `json:"corsPolicy,omitempty"`
	Links                 *IdentityZoneLinks      `json:"links,omitempty"`
	Prompts               []Prompt                `json:"prompts,omitempty"`
	IDPDiscoveryEnabled   *bool                   `json:"idpDiscoveryEnabled,omitempty"`
	Branding              *Branding               `json:"branding,omitempty"`
	AccountChooserEnabled *bool                   `json:"accountChooserEnabled,omitempty"`
	UserConfig            *IdentityZoneUserConfig `json:"userConfig,omitempty"`
	MFAConfig             *IdentityZoneMFAConfig  `json:"mfaConfig,omitempty"`
}
