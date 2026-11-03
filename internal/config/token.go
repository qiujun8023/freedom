package config

import "crypto/rand"

// tokenAlphabet 是大小写字母和数字，去掉了容易混淆的 0 O o 1 l I。
const tokenAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz"

// TokenLength 是 NewToken 生成的长度，约 93 bit。
const TokenLength = 16

// NewToken 生成一个随机 token。
func NewToken() string {
	b := make([]byte, TokenLength)
	for i := range b {
		b[i] = tokenAlphabet[randIndex(len(tokenAlphabet))]
	}
	return string(b)
}

// randIndex 返回 [0, n) 内均匀分布的随机数，丢弃会造成偏差的取值。
func randIndex(n int) int {
	limit := 256 - 256%n
	var buf [1]byte
	for {
		if _, err := rand.Read(buf[:]); err != nil {
			panic(err)
		}
		if int(buf[0]) < limit {
			return int(buf[0]) % n
		}
	}
}
