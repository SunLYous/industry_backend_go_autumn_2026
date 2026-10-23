package main

func rotateRunes(s string, shift int) string {
	if s == "" {
		return ""
	}

	runes := []rune(s)
	n := len(runes)

	k := ((shift % n) + n) % n
	if k == 0 {
		return string(runes)
	}

	return string(runes[k:]) + string(runes[:k])
}
