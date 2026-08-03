package models

import (
	"strings"

	"code.cloudfoundry.org/bbs/format"
)

func (c *CachedDependency) Validate() error {
	var validationError ValidationError

	if c.GetFrom() == "" {
		validationError = validationError.Append(ErrInvalidField{"from"})
	}

	if c.GetTo() == "" {
		validationError = validationError.Append(ErrInvalidField{"to"})
	}

	if c.GetChecksumValue() != "" && c.GetChecksumAlgorithm() == "" {
		validationError = validationError.Append(ErrInvalidField{"checksum algorithm"})
	}

	if c.GetChecksumValue() == "" && c.GetChecksumAlgorithm() != "" {
		validationError = validationError.Append(ErrInvalidField{"checksum value"})
	}

	if c.GetChecksumValue() != "" && c.GetChecksumAlgorithm() != "" {
		if !contains([]string{"md5", "sha1", "sha256"}, strings.ToLower(c.GetChecksumAlgorithm())) {
			validationError = validationError.Append(ErrInvalidField{"invalid algorithm"})
		}
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func validateCachedDependencies(cachedDependencies []*CachedDependency) ValidationError {
	var validationError ValidationError

	if len(cachedDependencies) > 0 {
		for _, cacheDep := range cachedDependencies {
			err := cacheDep.Validate()
			if err != nil {
				validationError = validationError.Append(ErrInvalidField{"cached_dependency"})
				validationError = validationError.Append(err)
			}
		}
	}

	return validationError
}

func (c *CachedDependency) Version() format.Version {
	return format.V0
}
