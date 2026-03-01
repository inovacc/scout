package identity

import "fmt"

const luhnBase32 = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"

func codepoint32(b byte) int {
	switch {
	case 'A' <= b && b <= 'Z':
		return int(b - 'A')
	case '2' <= b && b <= '7':
		return int(b + 26 - '2')
	default:
		return -1
	}
}

// luhn32 returns a check digit for the string s, which should be composed
// of characters from the base32 alphabet.
func luhn32(s string) (rune, error) {
	factor := 1
	sum := 0
	const n = 32

	for i := range s {
		codepoint := codepoint32(s[i])
		if codepoint == -1 {
			return 0, fmt.Errorf("digit %q not valid in alphabet %q", s[i], luhnBase32)
		}
		addend := factor * codepoint
		if factor == 2 {
			factor = 1
		} else {
			factor = 2
		}
		addend = (addend / n) + (addend % n)
		sum += addend
	}
	remainder := sum % n
	checkCodepoint := (n - remainder) % n
	return rune(luhnBase32[checkCodepoint]), nil
}

// luhnify adds Luhn check digits to a 52-character base32 string,
// producing a 56-character string (4 groups of 13+1 check digit).
func luhnify(s string) (string, error) {
	if len(s) != 52 {
		return "", fmt.Errorf("unsupported string length %d", len(s))
	}

	res := make([]byte, 4*(13+1))
	for i := range 4 {
		p := s[i*13 : (i+1)*13]
		copy(res[i*(13+1):], p)
		l, err := luhn32(p)
		if err != nil {
			return "", err
		}
		res[(i+1)*13+i] = byte(l)
	}
	return string(res), nil
}

// unluhnify validates and removes Luhn check digits from a 56-character string,
// returning the 52-character base32 string.
func unluhnify(s string) (string, error) {
	if len(s) != 56 {
		return "", fmt.Errorf("%q: unsupported string length %d", s, len(s))
	}

	res := make([]byte, 52)
	for i := range 4 {
		p := s[i*(13+1) : (i+1)*(13+1)-1]
		copy(res[i*13:], p)
		l, err := luhn32(p)
		if err != nil {
			return "", err
		}
		if s[(i+1)*14-1] != byte(l) {
			return "", fmt.Errorf("%q: check digit incorrect", s)
		}
	}
	return string(res), nil
}

// chunkify splits a string into groups of 7 separated by dashes.
func chunkify(s string) string {
	chunks := len(s) / 7
	res := make([]byte, chunks*(7+1)-1)
	for i := range chunks {
		if i > 0 {
			res[i*(7+1)-1] = '-'
		}
		copy(res[i*(7+1):], s[i*7:(i+1)*7])
	}
	return string(res)
}

// unchunkify removes dashes and spaces from a device ID string.
func unchunkify(s string) string {
	out := make([]byte, 0, len(s))
	for i := range s {
		if s[i] != '-' && s[i] != ' ' {
			out = append(out, s[i])
		}
	}
	return string(out)
}
