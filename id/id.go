package id

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

type ID struct {
	value [16]byte
}

func New() (id ID) {
	_, err := rand.Read(id.value[:])
	if err != nil {
		panic("run out of entropy")
	}
	return
}

var b58table = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

const strlen = 22

func (id ID) String() string {
	table := []byte(b58table)
	n := new(big.Int).SetBytes(id.value[:])

	e := []byte{}

	zero := big.NewInt(0)
	base := big.NewInt(58)

	for n.Cmp(zero) != 0 {
		remainder := new(big.Int)
		n.DivMod(n, base, remainder)
		e = append(e, table[remainder.Int64()])
	}

	for len(e) < strlen {
		e = append(e, table[0])
	}

	// reverse
	for i, j := 0, len(e)-1; i < j; i, j = i+1, j-1 {
		e[i], e[j] = e[j], e[i]
	}

	return string(e)
}

func Parse(s string) (id ID, err error) {
	if len(s) != strlen {
		err = fmt.Errorf("invalid b58 string length (%v)", len(s))
		return
	}

	base := big.NewInt(58)
	result := big.NewInt(0)

	for _, r := range []byte(s) {
		idx := strings.IndexByte(b58table, r)
		if idx == -1 {
			err = fmt.Errorf("invalid byte inside b58 string (%v)", r)
			return
		}
		result.Mul(result, base)
		result.Add(result, big.NewInt(int64(idx)))
	}

	bytes := result.Bytes()

	padding := 16 - len(bytes)
	if padding > 0 {
		paddingBytes := make([]byte, padding)
		bytes = append(paddingBytes, bytes...)
	}

	copy(id.value[:], bytes)
	return
}

func (id ID) MarshalJSON() ([]byte, error) {
	return json.Marshal(id.String())
}

func (id *ID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	decodedID, err := Parse(s)
	if err != nil {
		return err
	}

	*id = decodedID
	return nil
}
