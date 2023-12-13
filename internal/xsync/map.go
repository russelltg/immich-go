package xsync

import "sync"

// Map is a concurrent safe map[K]T
type Map[K comparable, T any] struct {
	lo   sync.RWMutex
	data map[K]T
}

// Len returns the number of map elements
func (m *Map[K, T]) Len() int {
	m.lo.RLock()
	defer m.lo.RUnlock()
	return len(m.data)
}

// Get the element associated with the K
func (m *Map[K, T]) Get(key K) T {
	m.lo.RLock()
	defer m.lo.RUnlock()
	return m.data[key]
}

// Set the element with the K
func (m *Map[K, T]) Set(key K, v T) {
	m.lo.Lock()
	defer m.lo.Unlock()
	m.data[key] = v
}

// Delete the K element
func (m *Map[K, T]) Delete(key K) {
	m.lo.Lock()
	defer m.lo.Unlock()
	delete(m.data, key)
}

// Return the a list of keys
func (m *Map[K, T]) Keys() []K {
	m.lo.RLock()
	defer m.lo.RUnlock()
	keys := make([]K, len(m.data))
	i := 0
	for k := range m.data {
		keys[i] = k
		i++
	}
	return keys
}
