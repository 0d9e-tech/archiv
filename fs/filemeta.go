package fs

import (
	"encoding/json"

	"archiiv/id"
)

const (
	PermOwner = uint8(1 << iota)
	PermRead
	PermWrite
)

// FileMeta contains the metadata asociated with each file. It is saved in the
// 'meta' section
type FileMeta struct {
	Id        id.ID            `json:"id"`
	Type      string           `json:"type"`
	Perms     map[string]uint8 `json:"perms"`
	Hooks     []string         `json:"hooks"`
	CreatedBy string           `json:"createdBy"`
	CreatedAt uint64           `json:"createdAt"`
}

func ReadFileMeta(fs *Fs, file id.ID) (fm FileMeta, err error) {
	r, err := fs.OpenSection(file, "meta")
	if err != nil {
		return
	}
	defer r.Close()

	err = json.NewDecoder(r).Decode(&fm)
	return
}

func WriteFileMeta(fs *Fs, file id.ID, fm FileMeta) error {
	w, err := fs.CreateSection(file, "meta")
	if err != nil {
		return err
	}
	defer w.Close()

	enc := json.NewEncoder(w)
	return enc.Encode(fm)
}
