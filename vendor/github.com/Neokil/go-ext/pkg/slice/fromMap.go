package slice

import "fmt"

// FromMap takes a map and generates a slice using the given convert function.
func FromMap[I, V any, K comparable](a map[K]V, convert func(K, V) I) []I {
	result := []I{}

	for aK, aV := range a {
		result = append(result, convert(aK, aV))
	}

	return result
}

// FromMap takes a map and generates a slice using the given convert function. If the convert-function returns an error the ToMapWithErr will wrap this error and return it.
func FromMaWithErr[I, V any, K comparable](a map[K]V, convert func(K, V) (I, error)) ([]I, error) {
	result := []I{}

	for aK, aV := range a {
		r, err := convert(aK, aV)
		if err != nil {
			return nil, fmt.Errorf("failed to convert map-element: %w", err)
		}
		result = append(result, r)
	}

	return result, nil
}
