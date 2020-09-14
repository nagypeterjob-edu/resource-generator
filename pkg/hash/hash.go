package hash

import (
	"crypto/md5"
	"encoding/hex"
	"io"
)

// Calculate hash is used for md5 hash calculation for an arbitrary file
func CalculateHash(data io.Reader) (string, error) {
	var returnMD5String string

	hash := md5.New()
	if _, err := io.Copy(hash, data); err != nil {
		return returnMD5String, err
	}
	hashInBytes := hash.Sum(nil)[:16]
	returnMD5String = hex.EncodeToString(hashInBytes)
	return returnMD5String, nil
}
