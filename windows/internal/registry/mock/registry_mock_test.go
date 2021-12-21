package mock

import (
	"errors"
	"marketplace-yaga/windows/internal/registry"
	"testing"
)

func TestNewMockKeyWithConfigValidation(t *testing.T) {
	t.Run("provided empty relative path", func(t *testing.T) {
		_, err := NewMockKeyWithConfigValidation("")
		if !errors.Is(err, registry.ErrRelativePathUndef) {
			t.Error(err)
		}
	})

	t.Run("write and read", func(t *testing.T) {
		k, err := NewMockKeyWithConfigValidation("any/path/fits")
		if err != nil {
			t.Error(err)
		}

		test := struct {
			wantProperty string
			wantValue    string
		}{
			"propertyName",
			"propertyValue",
		}

		err = k.WriteStringProperty(test.wantProperty, test.wantValue)
		if err != nil {
			t.Error(err)
		}

		var got string
		got, err = k.ReadStringProperty(test.wantProperty)
		if err != nil {
			t.Error(err)
		}

		if got != test.wantValue {
			t.Errorf("write != receive, wrote: %v, we got: %v", test.wantValue, got)
		}
	})
}
