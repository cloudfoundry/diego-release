package encryption

import "fmt"

type keyManager struct {
	encryptionKey  Key
	decryptionKeys map[string]Key
}

type KeyManager interface {
	EncryptionKey() Key
	DecryptionKey(label string) Key
}

func NewKeyManager(encryptionKey Key, decryptionKeys []Key) (KeyManager, error) {
	decryptionKeyMap := map[string]Key{
		encryptionKey.Label(): encryptionKey,
	}

	for _, key := range decryptionKeys {
		if existingKey, ok := decryptionKeyMap[key.Label()]; ok && key != existingKey {
			return nil, fmt.Errorf("Multiple keys with the same label: %q", key.Label())
		}
		decryptionKeyMap[key.Label()] = key
	}

	return &keyManager{
		encryptionKey:  encryptionKey,
		decryptionKeys: decryptionKeyMap,
	}, nil
}

func (m *keyManager) EncryptionKey() Key {
	return m.encryptionKey
}

func (m *keyManager) DecryptionKey(label string) Key {
	return m.decryptionKeys[label]
}
