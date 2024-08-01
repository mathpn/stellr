package main

// LevenshteinDistance calculates the Levenshtein distance between two strings
func LevenshteinDistance(str1, str2 string) int {
	runeStr1 := []rune(str1)
	runeStr2 := []rune(str2)

	runeStr1len := len(runeStr1)
	runeStr2len := len(runeStr2)

	if runeStr1len == 0 {
		return runeStr2len
	} else if runeStr2len == 0 {
		return runeStr1len
	} else if str1 == str2 {
		return 0
	}

	v0 := make([]int, runeStr2len+1)
	v1 := make([]int, runeStr2len+1)

	for y := 0; y <= runeStr2len; y++ {
		v0[y] = y
	}

	var cost int
	for i := 0; i < runeStr1len; i++ {
		v1[0] = i + 1

		for j := 0; j < runeStr2len; j++ {
			if runeStr1[i] == runeStr2[j] {
				cost = 0
			} else {
				cost = 1
			}
			v1[j+1] = min(v1[j]+1, v0[j+1]+1, v0[j]+cost)
		}

		v0, v1 = v1, v0
	}

	return v0[runeStr2len]
}
