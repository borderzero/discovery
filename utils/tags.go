package utils

// TagMatchesFilter returns true if a given key-value
// pair matches a given filter of key-value options.
func TagMatchesFilter(
	key string,
	value string,
	filter map[string][]string,
) bool {
	for filterKey, filterValues := range filter {
		if key == filterKey {
			// we interpret empty filter values
			// as "match any value of the key"
			if len(filterValues) == 0 {
				return true
			}
			for _, filterValue := range filterValues {
				if value == filterValue {
					return true
				}
			}
		}
	}
	return false
}
