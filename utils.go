package main

func Contains(where []string, what string) bool {
	for _, item := range where {
		if item == what {
			return true
		}
	}
	return false
}
