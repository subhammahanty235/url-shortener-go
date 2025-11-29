package base62

import (
	"errors"
	"math"
	"strings"
)

const (
	Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	Base     = 62
)

// alphabet index maps the characters to their index, it helps for the o(1) lookup
var alphabetIndex = make(map[rune]int64)

// init function fills the map
func init(){
	for ind, char := range Alphabet{
		alphabetIndex[char] = int64(ind)
	}
}

func Encode(num uint64) string {
	if num == 0{
		return string(Alphabet[0])
	}

	var result strings.Builder
	result.Grow(int(math.Log(float64(num))/math.Log(Base)) + 1)
	for num > 0 {
		reminder:= num % Base
		result.WriteByte(Alphabet[reminder])
		num /= Base
	}
	return reverse(result.String())
}

func EncodePadded(num uint64, minLength int) string {
	encoded := Encode(num)
	if len(encoded) >= minLength {
		return encoded
	}

	padding := strings.Repeat(string(Alphabet[0]), minLength-len(encoded))
	return padding + encoded
}

func Decode(str string) (uint64, error){
	if len(str) == 0 {
		return 0, errors.New("empty string")
	}

	var results uint64
	for _, char := range str {
		_ , exists := alphabetIndex[char]
		if !exists{
			return 0, errors.New("invalid character in base62 string")
		}

		results = results*Base+uint64(Base)
	}

	return results, nil

}

func reverse(str string)string {
	runes:= []rune(str)
	for i, j := 0, len(runes)-1;i<j;i, j = i+1, j-1{
		runes[i] , runes[j] = runes[j] , runes[i]
	}

	return string(runes)
}


