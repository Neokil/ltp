package slice

// GroupBy returns a map of arrays generated from the original array.
// The key of the map is some function of the array element including the identity.
func GroupBy[K any, V comparable](a []K, key func(K) V) map[V][]K {
	m := make(map[V][]K)
	for _, element := range a {
		k := key(element)
		m[k] = append(m[k], element)
	}

	return m
}
