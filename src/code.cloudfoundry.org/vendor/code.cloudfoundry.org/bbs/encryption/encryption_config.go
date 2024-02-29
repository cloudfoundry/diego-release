package encryption

import "errors"

type EncryptionConfig struct {
	ActiveKeyLabel string            `json:"active_key_label"`
	EncryptionKeys map[string]string `json:"encryption_keys"`
}

func (ef *EncryptionConfig) Parse() (Key, []Key, error) {
	if len(ef.EncryptionKeys) == 0 {
		return nil, nil, errors.New("Must have at least one encryption key set")
	}

	if len(ef.ActiveKeyLabel) == 0 {
		return nil, nil, errors.New("Must select an active encryption key")
	}

	var encryptionKey Key

	labelsToKeys := map[string]Key{}

	for label, phrase := range ef.EncryptionKeys {
		key, err := NewKey(label, phrase)
		if err != nil {
			return nil, nil, err
		}
		labelsToKeys[label] = key
	}

	encryptionKey, ok := labelsToKeys[ef.ActiveKeyLabel]
	if !ok {
		return nil, nil, errors.New("The selected active key must be listed on the encryption keys flag")
	}

	keys := []Key{}
	for _, v := range labelsToKeys {
		keys = append(keys, v)
	}

	return encryptionKey, keys, nil
}
