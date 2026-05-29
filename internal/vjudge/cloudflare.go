package vjudge

import (
	"crypto/sha1"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
)

const cfPoWPrefixBits = 16

func solvePoW(suffix string) string {
	for n := 0; n < 500000; n++ {
		input := strconv.Itoa(n) + "_" + suffix
		hash := sha1.Sum([]byte(input))
		hashHex := hex.EncodeToString(hash[:])
		if hashHex[:cfPoWPrefixBits/4] == strings.Repeat("0", cfPoWPrefixBits/4) {
			return strconv.Itoa(n) + "_" + suffix
		}
	}
	return ""
}

func cfAPISign(methodName string, params map[string]string, secret string) (string, string) {
	randHex := fmt.Sprintf("%06x", rand.Intn(0xFFFFFF))

	var parts []string
	for k, v := range params {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	queryStr := strings.Join(parts, "&")

	hashInput := randHex + "/" + methodName + "?" + queryStr + "#" + secret
	hash := sha512.Sum512([]byte(hashInput))
	return randHex, hex.EncodeToString(hash[:])
}
