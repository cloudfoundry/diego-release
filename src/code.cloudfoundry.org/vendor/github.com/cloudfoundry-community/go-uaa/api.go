package uaa

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"

	pc "github.com/cloudfoundry-community/go-uaa/passwordcredentials"
	"golang.org/x/oauth2"
	cc "golang.org/x/oauth2/clientcredentials"
)

//go:generate go run ./generator/generator.go

// API is a client to the UAA API.
type API struct {
	Client                    *http.Client
	baseClient                *http.Client
	baseTransport             http.RoundTripper
	TargetURL                 *url.URL
	redirectURL               *url.URL
	skipSSLValidation         bool
	verbose                   bool
	zoneID                    string
	userAgent                 string
	token                     *oauth2.Token
	target                    string
	mode                      mode
	clientID                  string
	clientSecret              string
	username                  string
	password                  string
	authorizationCode         string
	refreshToken              string
	tokenFormat               TokenFormat
	clientCredentialsConfig   *cc.Config
	passwordCredentialsConfig *pc.Config
	oauthConfig               *oauth2.Config
}

// TokenFormat is the format of a token.
type TokenFormat int

// Valid TokenFormat values.
const (
	OpaqueToken TokenFormat = iota
	JSONWebToken
)

func (t TokenFormat) String() string {
	if t == OpaqueToken {
		return "opaque"
	}
	if t == JSONWebToken {
		return "jwt"
	}
	return ""
}

type mode int

const (
	custom mode = iota
	token
	clientcredentials
	passwordcredentials
	authorizationcode
	refreshtoken
)

type Option interface {
	Apply(a *API)
}

type AuthenticationOption interface {
	ApplyAuthentication(a *API)
}

func New(target string, authOpt AuthenticationOption, opts ...Option) (*API, error) {
	a := &API{
		target: target,
		mode:   custom,
	}
	authOpt.ApplyAuthentication(a)
	defaultClient := &http.Client{Transport: http.DefaultTransport}
	defaultClientOption := WithClient(defaultClient)
	defaultUserAgentOption := WithUserAgent("go-uaa")
	opts = append([]Option{defaultClientOption, defaultUserAgentOption}, opts...)
	for _, option := range opts {
		option.Apply(a)
	}
	err := a.configure()
	if err != nil {
		return nil, err
	}

	return a, nil
}

func (a *API) Token(ctx context.Context) (*oauth2.Token, error) {
	if _, ok := ctx.Value(oauth2.HTTPClient).(*http.Client); !ok {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, a.baseClient)
	}

	switch a.mode {
	case token:
		if !a.token.Valid() {
			return nil, errors.New("you have supplied an empty, invalid, or expired token to go-uaa")
		}
		return a.token, nil
	case clientcredentials:
		if a.clientCredentialsConfig == nil {
			return nil, errors.New("you have supplied invalid client credentials configuration to go-uaa")
		}
		return a.clientCredentialsConfig.Token(ctx)
	case authorizationcode:
		if a.oauthConfig == nil {
			return nil, errors.New("you have supplied invalid authorization code configuration to go-uaa")
		}
		tokenFormatParam := oauth2.SetAuthURLParam("token_format", a.tokenFormat.String())
		responseTypeParam := oauth2.SetAuthURLParam("response_type", "token")

		return a.oauthConfig.Exchange(ctx, a.authorizationCode, tokenFormatParam, responseTypeParam)
	case refreshtoken:
		if a.oauthConfig == nil {
			return nil, errors.New("you have supplied invalid refresh token configuration to go-uaa")
		}

		tokenSource := a.oauthConfig.TokenSource(ctx, &oauth2.Token{
			RefreshToken: a.refreshToken,
		})

		token, err := tokenSource.Token()
		return token, requestErrorFromOauthError(err)
	case passwordcredentials:
		token, err := a.passwordCredentialsConfig.TokenSource(ctx).Token()
		return token, requestErrorFromOauthError(err)
	}
	return nil, errors.New("your configuration provides no way for go-uaa to get a token")
}

func (a *API) baseTransportIsNil() bool {
	if a.baseTransport == nil || reflect.ValueOf(a.baseTransport).IsNil() {
		return true
	}
	return false
}

func (a *API) configure() error {
	err := a.configureTarget()
	if err != nil {
		return err
	}
	if a.baseClient == nil {
		return errors.New("please ensure you pass a non-nil client to uaa.WithClient, or remove the uaa.WithClient option")
	}
	if a.baseTransportIsNil() {
		a.baseTransport = a.baseClient.Transport
	}
	if a.baseTransportIsNil() {
		a.baseTransport = http.DefaultTransport
	}

	a.ensureTransport(a.baseClient.Transport)
	wrappedTransport := &uaaTransport{
		base:           a.baseClient.Transport,
		LoggingEnabled: a.verbose,
	}
	a.baseClient.Transport = wrappedTransport
	switch a.mode {
	case token:
		err = a.configureToken()
	case clientcredentials:
		a.configureClientCredentials()
	case passwordcredentials:
		a.configurePasswordCredentials()
	case authorizationcode:
		err = a.configureAuthorizationCode()
	case refreshtoken:
		err = a.configureRefreshToken()
	case custom:
		if a.Client == nil {
			a.Client = a.baseClient
		}
	default:
		return errors.New("please ensure you pass an AuthenticationOption (e.g. WithClientCredentials, WithPasswordCredentials, WithAuthorizationCode, WithRefreshToken, WithToken) to New(), or manually construct a uaa.API and set uaa.API.Client")
	}
	if err != nil {
		return err
	}
	if a.Client == nil {
		return errors.New("Client is nil; please ensure you pass an AuthenticationOption (e.g. WithClientCredentials, WithPasswordCredentials, WithAuthorizationCode, WithRefreshToken, WithToken) to New(), or manually set Client")
	}
	a.ensureTransport(a.Client.Transport)
	return nil
}

func (a *API) configureTarget() error {
	if a.TargetURL != nil {
		return nil
	}
	if a.target == "" && a.TargetURL == nil {
		return errors.New("the target is missing")
	}
	u, err := BuildTargetURL(a.target)
	if err != nil {
		return err
	}
	a.TargetURL = u
	return nil
}

type withClient struct {
	client *http.Client
}

func WithClient(client *http.Client) Option {
	return &withClient{client: client}
}

func (w *withClient) Apply(a *API) {
	a.baseClient = w.client
}

type withTransport struct {
	transport http.RoundTripper
}

func WithTransport(transport http.RoundTripper) Option {
	return &withTransport{transport: transport}
}

func (w *withTransport) Apply(a *API) {
	a.baseTransport = w.transport
}

type withSkipSSLValidation struct {
	skipSSLValidation bool
}

func WithSkipSSLValidation(skipSSLValidation bool) Option {
	return &withSkipSSLValidation{skipSSLValidation: skipSSLValidation}
}

func (w *withSkipSSLValidation) Apply(a *API) {
	a.skipSSLValidation = w.skipSSLValidation
}

type withUserAgent struct {
	userAgent string
}

func WithUserAgent(userAgent string) Option {
	return &withUserAgent{userAgent: userAgent}
}

func (w *withUserAgent) Apply(a *API) {
	a.userAgent = w.userAgent
}

type withZoneID struct {
	zoneID string
}

func WithZoneID(zoneID string) Option {
	return &withZoneID{zoneID: zoneID}
}

func (w *withZoneID) Apply(a *API) {
	a.zoneID = w.zoneID
}

type withVerbosity struct {
	verbose bool
}

func WithVerbosity(verbose bool) Option {
	return &withVerbosity{verbose: verbose}
}

func (w *withVerbosity) Apply(a *API) {
	a.verbose = w.verbose
}

type withClientCredentials struct {
	clientID     string
	clientSecret string
	tokenFormat  TokenFormat
}

func WithClientCredentials(clientID string, clientSecret string, tokenFormat TokenFormat) AuthenticationOption {
	return &withClientCredentials{clientID: clientID, clientSecret: clientSecret, tokenFormat: tokenFormat}
}

func (w *withClientCredentials) ApplyAuthentication(a *API) {
	a.mode = clientcredentials
	a.clientID = w.clientID
	a.clientSecret = w.clientSecret
	a.tokenFormat = w.tokenFormat
}

func (a *API) configureClientCredentials() {
	tokenURL := urlWithPath(*a.TargetURL, "/oauth/token")
	v := url.Values{}
	v.Add("token_format", a.tokenFormat.String())
	c := &cc.Config{
		ClientID:       a.clientID,
		ClientSecret:   a.clientSecret,
		TokenURL:       tokenURL.String(),
		EndpointParams: v,
		AuthStyle:      oauth2.AuthStyleInHeader,
	}
	a.clientCredentialsConfig = c
	a.Client = c.Client(context.WithValue(
		context.Background(),
		oauth2.HTTPClient,
		a.baseClient,
	))
}

type withPasswordCredentials struct {
	clientID     string
	clientSecret string
	username     string
	password     string
	tokenFormat  TokenFormat
}

func WithPasswordCredentials(clientID string, clientSecret string, username string, password string, tokenFormat TokenFormat) AuthenticationOption {
	return &withPasswordCredentials{
		clientID:     clientID,
		clientSecret: clientSecret,
		username:     username,
		password:     password,
		tokenFormat:  tokenFormat,
	}
}

func (w *withPasswordCredentials) ApplyAuthentication(a *API) {
	a.mode = passwordcredentials
	a.clientID = w.clientID
	a.clientSecret = w.clientSecret
	a.username = w.username
	a.password = w.password
	a.tokenFormat = w.tokenFormat
}

func (a *API) configurePasswordCredentials() {
	tokenURL := urlWithPath(*a.TargetURL, "/oauth/token")
	v := url.Values{}
	v.Add("token_format", a.tokenFormat.String())
	c := &pc.Config{
		ClientID:     a.clientID,
		ClientSecret: a.clientSecret,
		Username:     a.username,
		Password:     a.password,
		Endpoint: oauth2.Endpoint{
			TokenURL: tokenURL.String(),
		},
		EndpointParams: v,
	}
	a.passwordCredentialsConfig = c
	a.Client = c.Client(context.WithValue(
		context.Background(),
		oauth2.HTTPClient,
		a.baseClient))
}

type withAuthorizationCode struct {
	clientID          string
	clientSecret      string
	authorizationCode string
	redirectURL       *url.URL
	tokenFormat       TokenFormat
}

func WithAuthorizationCode(clientID string, clientSecret string, authorizationCode string, tokenFormat TokenFormat, redirectURL *url.URL) AuthenticationOption {
	return &withAuthorizationCode{
		clientID:          clientID,
		clientSecret:      clientSecret,
		authorizationCode: authorizationCode,
		tokenFormat:       tokenFormat,
		redirectURL:       redirectURL,
	}
}

func (w *withAuthorizationCode) ApplyAuthentication(a *API) {
	a.mode = authorizationcode
	a.clientID = w.clientID
	a.clientSecret = w.clientSecret
	a.authorizationCode = w.authorizationCode
	a.tokenFormat = w.tokenFormat
	a.redirectURL = w.redirectURL
}

func (a *API) configureAuthorizationCode() error {
	tokenURL := urlWithPath(*a.TargetURL, "/oauth/token")
	c := &oauth2.Config{
		ClientID:     a.clientID,
		ClientSecret: a.clientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL:  tokenURL.String(),
			AuthStyle: oauth2.AuthStyleInHeader,
		},
		RedirectURL: a.redirectURL.String(),
	}
	a.oauthConfig = c
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, a.baseClient)

	if !a.token.Valid() {
		t, err := a.Token(context.Background())
		if err != nil {
			return requestErrorFromOauthError(err)
		}
		a.token = t
	}

	a.Client = c.Client(ctx, a.token)
	return nil
}

type withRefreshToken struct {
	clientID     string
	clientSecret string
	refreshToken string
	tokenFormat  TokenFormat
}

func WithRefreshToken(clientID string, clientSecret string, refreshToken string, tokenFormat TokenFormat) AuthenticationOption {
	return &withRefreshToken{
		clientID:     clientID,
		clientSecret: clientSecret,
		refreshToken: refreshToken,
		tokenFormat:  tokenFormat,
	}
}

func (w *withRefreshToken) ApplyAuthentication(a *API) {
	a.mode = refreshtoken
	a.clientID = w.clientID
	a.clientSecret = w.clientSecret
	a.refreshToken = w.refreshToken
	a.tokenFormat = w.tokenFormat
}

func (a *API) configureRefreshToken() error {
	tokenURL := urlWithPath(*a.TargetURL, "/oauth/token")
	query := tokenURL.Query()
	query.Set("token_format", a.tokenFormat.String())
	tokenURL.RawQuery = query.Encode()
	c := &oauth2.Config{
		ClientID:     a.clientID,
		ClientSecret: a.clientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL:  tokenURL.String(),
			AuthStyle: oauth2.AuthStyleInHeader,
		},
	}
	a.oauthConfig = c
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, a.baseClient)

	if !a.token.Valid() {
		t, err := a.Token(context.Background())
		if err != nil {
			return err
		}
		a.token = t
	}

	a.Client = c.Client(ctx, a.token)
	return nil
}

type withToken struct {
	token *oauth2.Token
}

func WithToken(token *oauth2.Token) AuthenticationOption {
	return &withToken{token: token}
}

func (w *withToken) ApplyAuthentication(a *API) {
	a.mode = token
	a.token = w.token
}

func (a *API) configureToken() error {
	if !a.token.Valid() {
		return errors.New("access token is not valid, or is expired")
	}

	tokenClient := &http.Client{
		Transport: &tokenTransport{
			underlyingTransport: a.baseClient.Transport,
			token:               *a.token,
		},
	}

	a.Client = tokenClient
	return nil
}

type tokenTransport struct {
	underlyingTransport http.RoundTripper
	token               oauth2.Token
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", fmt.Sprintf("%s %s", t.token.Type(), t.token.AccessToken))
	return t.underlyingTransport.RoundTrip(req)
}

type withNoAuthentication struct {
}

func WithNoAuthentication() AuthenticationOption {
	return &withNoAuthentication{}
}

func (w *withNoAuthentication) ApplyAuthentication(a *API) {
	a.mode = custom
}
