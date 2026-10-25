package handler

import "crypto/subtle"

func subtleConstant(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
