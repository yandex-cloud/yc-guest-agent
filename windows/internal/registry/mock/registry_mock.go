package mock

import (
	"marketplace-yaga/windows/internal/registry"
)

type keyMock struct{ data map[string]string }

var mockKey keyMock

// NewMockKeyWithConfigValidation va
// Since we'll run unit tests on linux we need to validate config somehow
// to mock obvious errors, which will be passed normally if have had windows
// or which is transient with serial config for example BackoffInterval.
func NewMockKeyWithConfigValidation(relativePath string) (registry.StringPropReaderWriter, error) {
	if relativePath == "" {
		return nil, registry.ErrRelativePathUndef
	}

	return registry.OpenKeyWithOpener(func(uint32) (registry.StringPropReadWriteCloser, error) {
		return &mockKey, nil
	}), nil
}

func (m *keyMock) GetStringValue(name string) (string, uint32, error) {
	val, ok := m.data[name]
	if !ok {
		return "", 0, registry.ErrNotExist
	}

	// property type SZ = 1
	return val, 1, nil
}

func (m *keyMock) SetStringValue(name string, value string) (err error) {
	if m.data == nil {
		m.data = make(map[string]string)
	}
	m.data[name] = value

	return nil
}

func (m *keyMock) Close() (err error) {
	return
}
