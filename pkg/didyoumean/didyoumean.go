package didyoumean

import "iter"

// Suggest returns the closest match from candidates to the given input, or an
// empty string if no candidate is within a reasonable edit distance. The
// threshold is half the length of the input (minimum 2).
func Suggest(input string, candidates iter.Seq[string]) string {
	if len(input) == 0 {
		return ""
	}
	if candidates == nil {
		return ""
	}

	threshold := len(input) / 2
	if threshold < 2 {
		threshold = 2
	}

	best := ""
	bestDist := threshold + 1
	for c := range candidates {
		d := levenshtein(input, c)
		if d < bestDist {
			bestDist = d
			best = c
		}
	}
	return best
}

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)

	// Use a single row instead of a full matrix.
	prev := make([]int, lb+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr := make([]int, lb+1)
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			ins := curr[j-1] + 1
			del := prev[j] + 1
			sub := prev[j-1] + cost
			curr[j] = min(ins, del, sub)
		}
		prev = curr
	}
	return prev[lb]
}
