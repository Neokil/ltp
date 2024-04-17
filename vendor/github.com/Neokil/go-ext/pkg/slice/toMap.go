package slice

import "fmt"

// ToMap takes a slice and generates a map using the given convert function.
func ToMap[I, V any, K comparable](a []I, convert func(I) (K, V)) map[K]V {
	result := map[K]V{}

	for _, ae := range a {
		k, v := convert(ae)
		result[k] = v
	}

	return result
}

// ToMapWithErr takes a slice and generates a map using the given convert function. If the convert-function returns an error the ToMapWithErr will wrap this error and return it.
func ToMapWithErr[I, V any, K comparable](a []I, convert func(I) (K, V, error)) (map[K]V, error) {
	result := map[K]V{}

	for _, ae := range a {
		k, v, err := convert(ae)
		if err != nil {
			return nil, fmt.Errorf("failed to convert slice-element: %w", err)
		}
		result[k] = v
	}

	return result, nil
}
