package entities

// trueCount counts the number of true values in a list of bools and returns the count
func trueCount(bools ...bool) int {
	var count int
	for _, b := range bools {
		if b {
			count++
		}
	}
	return count
}
