package errors

import (
	"errors"
	"fmt"
)

func NewNetworkError(e error) error {
	return errors.New(fmt.Sprintf("Error connecting to the targeted API: %#v. Please validate your target and retry your request.", e.Error()))
}

func NewAuthServerNetworkError(e error) error {
	return errors.New(fmt.Sprintf("Error connecting to the auth server: %#v. Please validate your target and retry your request.", e.Error()))
}

func NewCatchAllError() error {
	return errors.New("The targeted API was unable to perform the request. Please validate and retry your request.")
}

func NewFailedToImportError() error {
	return errors.New("One or more credentials failed to import.")
}

func NewRevokedTokenError() error {
	return errors.New("You are not currently authenticated. Please log in to continue.")
}

func NewFileLoadError() error {
	return errors.New("A referenced file could not be opened. Please validate the provided filenames and permissions, then retry your request.")
}

func NewMissingGetParametersError() error {
	return errors.New("A name or ID must be provided. Please update and retry your request.")
}

func NewMissingDeleteParametersError() error {
	return errors.New("A name or path must be provided. Please update and retry your request.")
}

func NewBulkDeleteFailureError() error {
	return errors.New("Some or all of the credential under the provided path could not be deleted. Please refer to the error output.")
}

func NewMissingInterpolateParametersError() error {
	return errors.New("A file to interpolate must be provided. Please add a file flag and try again.")
}

func NewMixedAuthorizationParametersError() error {
	return errors.New("Client, password, SSO and/or SSO passcode credentials may not be combined. Please update and retry your request with a single login method.")
}

func NewPasswordAuthorizationParametersError() error {
	return errors.New("The combination of parameters in the request is not allowed. Please validate your input and retry your request.")
}

func NewClientAuthorizationParametersError() error {
	return errors.New("Both client name and client secret must be provided to authenticate. Please update and retry your request.")
}

func NewRefreshError() error {
	return errors.New("You are not currently authenticated. Please log in to continue.")
}

func NewNoMatchingCredentialsFoundError() error {
	return errors.New("No credentials exist which match the provided parameters.")
}

func NewSetEmptyTypeError() error {
	return errors.New("A type must be specified when setting a credential. Valid types include 'value', 'json', 'password', 'user', 'certificate', 'ssh' and 'rsa'.")
}

func NewGenerateEmptyTypeError() error {
	return errors.New("A type must be specified when generating a credential. Valid types include 'password', 'user', 'certificate', 'ssh' and 'rsa'.")
}

func NewNoApiUrlSetError() error {
	return errors.New("An API target is not set. Please target the location of your server with `credhub api --server api.example.com` to continue.")
}

func NewInvalidImportYamlError() error {
	return errors.New("The referenced file does not contain valid yaml structure. Please update and retry your request.")
}

func NewInvalidImportJSONError() error {
	return errors.New("The referenced file does not contain valid json structure. Please update and retry your request.")
}

func NewNoCredentialsTagError() error {
	return errors.New("The referenced import file does not begin with the key 'credentials'. The import file must contain a list of credentials under the key 'credentials'. Please update and retry your request.")
}

func NewGetVersionAndKeyError() error {
	return errors.New("The --versions flag and --key flag are incompatible.")
}

func NewGetVersionsAndIDIncompatibleParametersError() error {
	return errors.New("The --versions flag and --id flag are incompatible.")
}

func NewOutputJSONAndQuietError() error {
	return errors.New("The --output-json flag and --quiet flag are incompatible.")
}

func NewUserNameOnlyValidForUserType() error {
	return errors.New("Username parameter is not valid for this credential type.")
}

func NewUAAError(err error) error {
	return errors.New("UAA error: " + err.Error())
}

func NewInvalidJSONMetadataError() error {
	return errors.New("The argument for --metadata is not a valid json object. Please update and retry your request.")
}

func NewServerDoesNotSupportMetadataError() error {
	return errors.New("The --metadata flag is not supported for this version of the credhub server (requires >= 2.6.x). Please remove the flag and retry your request.")
}
