package slice

// Unique returns distinct elements from K based on custom uniqueness key.
func Unique[K any, V comparable](a []K, key func(K) V) []K {
	res := []K{}

	g := GroupBy(a, key)
	for _, v := range g {
		res = append(res, v[0])
	}

	return res
}
