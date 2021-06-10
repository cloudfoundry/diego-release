package consuladapter

import "fmt"

func NewKeyNotFoundError(key string) error {
	return KeyNotFoundError(key)
}

type KeyNotFoundError string

func (e KeyNotFoundError) Error() string {
	return fmt.Sprintf("key not found: '%s'", string(e))
}

func NewPrefixNotFoundError(prefix string) error {
	return PrefixNotFoundError(prefix)
}

type PrefixNotFoundError string

func (e PrefixNotFoundError) Error() string {
	return fmt.Sprintf("prefix not found: '%s'", string(e))
}
