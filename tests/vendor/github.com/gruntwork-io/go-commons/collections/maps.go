package collections

import "sort"

// Merge all the maps into one. Sadly, Go has no generics, so this is only defined for string to interface maps.
func MergeMaps(maps ... map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}

	for _, currMap := range maps {
		for key, value := range currMap {
			out[key] = value
		}
	}

	return out
}

// Return the keys for the given map, sorted alphabetically
func Keys(m map[string]string) []string {
	out := []string{}

	for key, _ := range m {
		out = append(out, key)
	}

	sort.Strings(out)

	return out
}