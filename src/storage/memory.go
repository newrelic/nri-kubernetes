package storage

import (
	"fmt"
	"reflect"
	"time"
)

// MemoryStorage stores data in memory.
type MemoryStorage struct {
	values map[string]dataWithTimestamp
}

type dataWithTimestamp struct {
	timestamp int64
	data      interface{}
}

func (m *MemoryStorage) Write(key string, value interface{}) error {
	if m.values == nil {
		m.values = map[string]dataWithTimestamp{}
	}

	if value == nil {
		return fmt.Errorf("given cache value is nil")
	}

	if kind := reflect.ValueOf(value).Kind(); kind != reflect.Ptr {
		return fmt.Errorf("given value is not a pointer, got %q", kind)
	}

	m.values[key] = dataWithTimestamp{
		timestamp: time.Now().Unix(),
		data:      value,
	}

	return nil
}

func (m *MemoryStorage) Read(key string, returnValue interface{}) (int64, error) {
	if kind := reflect.ValueOf(returnValue).Kind(); kind != reflect.Ptr {
		return 0, fmt.Errorf("given return value is not a pointer, got %q", kind)
	}

	value, ok := m.values[key]
	if !ok {
		return 0, fmt.Errorf("key not found: %s", key)
	}

	// Saved value is guaranteed to be a pointer, so dereference it with Elem() here,
	// so we can write it's value to inout returnValue.
	cachedValue := reflect.ValueOf(value.data).Elem()

	// Dereference returnValue which must be a pointer and write it's content with cached value.
	reflect.ValueOf(returnValue).Elem().Set(cachedValue)

	return value.timestamp, nil
}

func (m *MemoryStorage) Delete(key string) error {
	if _, ok := m.values[key]; !ok {
		return fmt.Errorf("key not found: %s", key)
	}

	delete(m.values, key)

	return nil
}
