package utils

// KVMatchesFilters returns true if the given key-value
// pairs match the given inclusion and exclusion filters.
func KVMatchesFilters(
	kv map[string]string,
	inclusion map[string][]string,
	exclusion map[string][]string,
) bool {
	included := (inclusion == nil)
	excluded := false

	if inclusion != nil {
		for key, value := range kv {
			if kvMatchesFilter(
				key,
				value,
				inclusion,
			) {
				included = true
				break
			}
		}
	}

	if exclusion != nil {
		for key, value := range kv {
			if kvMatchesFilter(
				key,
				value,
				inclusion,
			) {
				excluded = true
				break
			}
		}
	}

	return included && !excluded
}

// kvMatchesFilter returns true if a given key-value
// pair matches a given filter of key-value options.
func kvMatchesFilter(
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
