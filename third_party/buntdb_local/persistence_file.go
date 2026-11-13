package buntdb

import (
	"crypto/rand"
	"encoding/hex"
	"os"
)

func createPersistenceTemp(path string) (*os.File, error) {
	for attempt := 0; attempt < 8; attempt++ {
		var random [8]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, err
		}
		name := path + ".compact-" + hex.EncodeToString(random[:])
		file, err := openPersistenceFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
		if os.IsExist(err) {
			continue
		}
		return file, err
	}
	return nil, os.ErrExist
}
