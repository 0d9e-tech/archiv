// Package fs handles interactions with the system fs and how the data is
// serialised. It exposes a simple API that is used by the endpoints
package fs

// the directory tree is modeled using the Records structs
// they are reference counted and thus are forbidden to form cycles
//
// Records are saved as $fs_root/$id
//
// Records contain sections saved as $fs_root/$id.$section The file payload
// is saved in the 'data' section. metadata is in 'meta'. hooks can create own
// sections
//
// External function, which take IDs as inputs are thread safe. Internal
// functions, which take pointers to records instead are not thread safe.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"archiiv/id"
)

const (
	sectionPattern      = `[a-zA-Z0-9_-]+`
	idPattern           = `[1-9A-HJ-NP-Za-km-z]{22}`
	fileInFsRootPattern = idPattern + `(\.` + sectionPattern + `)?`

	onlyIDPattern           = `^` + idPattern + `$`
	onlyFileInFsRootPattern = `^` + fileInFsRootPattern + `$`
	onlySectionPattern      = `^` + sectionPattern + `$`
)

var (
	onlySectionPatternRegex      = regexp.MustCompile(onlySectionPattern)
	onlyFileInFsRootPatternRegex = regexp.MustCompile(onlyFileInFsRootPattern)
)

type record struct {
	Children []id.ID    `json:"children,omitempty"`
	IsDir    bool       `json:"is_dir"`
	Name     string     `json:"name"`
	id       id.ID      `json:"-"`
	refs     uint       `json:"-"`
	mutex    sync.Mutex `json:"-"`
}

func (r *record) lock() {
	r.mutex.Lock()
}

func (r *record) unlock() {
	r.mutex.Unlock()
}

type Fs struct {
	lock     sync.RWMutex
	records  map[id.ID]*record
	root     id.ID
	basePath string
}

func (fs *Fs) record(u id.ID) (*record, error) {
	fs.lock.RLock()
	defer fs.lock.Unlock()
	r, e := fs.records[u]
	if e {
		return nil, errors.New("id doesn't exist")
	}
	return r, nil
}

func (fs *Fs) setRecord(r *record) {
	fs.lock.Lock()
	defer fs.lock.Unlock()
	fs.records[r.id] = r
}

func (fs *Fs) path(p string) string {
	// TODO(marek) sanitize paths
	return filepath.Join(fs.basePath, p)
}

func (fs *Fs) writeRecord(r *record) error {
	f, err := os.Create(fs.path(r.id.String()))
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	return enc.Encode(r)
}

func (fs *Fs) newRecord(parent *record, name string, dir bool) (*record, error) {
	child := new(record)
	child.Children = []id.ID{}
	child.id = id.New()
	child.Name = name
	child.refs = 1
	child.IsDir = dir

	fs.setRecord(child)

	for _, e := range parent.Children {
		if e == child.id {
			return nil, errors.New("child already there")
		}
	}

	parent.Children = append(parent.Children, child.id)

	return child, fs.writeRecord(child)
}

// return new slice that does not contain v
func removeID(s []id.ID, v id.ID) ([]id.ID, error) {
	i := 0
	pos := -1
	for ; i < len(s); i++ {
		if s[i] == v {
			pos = i
			break
		}
	}

	for ; i < len(s); i++ {
		if s[i] == v {
			return s, errors.New("duplicite id")
		}
	}

	if pos == -1 {
		return s, errors.New("id not found")
	}

	// swap remove
	s[pos] = s[len(s)-1]
	return s[:len(s)-1], nil
}

func checkSectionNameSanity(section string) error {
	if !onlySectionPatternRegex.MatchString(section) {
		return errors.New("section name is not sane")
	}
	return nil
}

func (fs *Fs) getSectionFileName(file id.ID, section string) string {
	return fs.path(file.String() + "." + section)
}

func (fs *Fs) deleteRecord(r *record) error {
	for _, u := range r.Children {
		err := fs.Unmount(r.id, u)
		if err != nil {
			return err
		}
	}

	entries, err := os.ReadDir(fs.basePath)
	if err != nil {
		return err
	}

	idStr := r.id.String()
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), idStr) {
			err = os.Remove(e.Name())
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (fs *Fs) GetRoot() id.ID {
	return fs.root
}

func (fs *Fs) GetChildren(u id.ID) ([]id.ID, error) {
	return fs.records[u].Children, nil
}

func (fs *Fs) Mkdir(parentID id.ID, name string) (id.ID, error) {
	parent, err := fs.record(parentID)
	if err != nil {
		return id.ID{}, nil
	}

	r, err := fs.newRecord(parent, name, true)
	return r.id, err
}

func (fs *Fs) Touch(parentID id.ID, name string) (id.ID, error) {
	parent, err := fs.record(parentID)
	if err != nil {
		return id.ID{}, err
	}

	r, err := fs.newRecord(parent, name, false)
	return r.id, err
}

func (fs *Fs) Unmount(parentID id.ID, childID id.ID) error {
	parent, err := fs.record(parentID)
	if err != nil {
		return err
	}

	parent.lock()
	defer parent.unlock()

	parent.Children, err = removeID(parent.Children, childID)
	if err != nil {
		return err
	}

	err = fs.writeRecord(parent)
	if err != nil {
		return err
	}

	child, err := fs.record(childID)
	if err != nil {
		return err
	}

	child.lock()
	defer child.unlock()

	child.refs--
	if child.refs == 0 {
		return fs.deleteRecord(child)
	}

	return nil
}

func (fs *Fs) Mount(parent id.ID, newChild id.ID) error {
	child, err := fs.record(newChild)
	if err != nil {
		return err
	}

	rec, err := fs.record(parent)
	if err != nil {
		return err
	}
	rec.lock()
	defer rec.unlock()

	for _, child := range rec.Children {
		if child == newChild {
			return errors.New("child with this id already exists")
		}
	}

	rec.Children = append(rec.Children, newChild)

	child.lock()
	child.refs++
	child.unlock()

	return fs.writeRecord(rec)
}

func (fs *Fs) OpenSection(id id.ID, section string) (io.ReadCloser, error) {
	err := checkSectionNameSanity(section)
	if err != nil {
		return nil, err
	}
	return os.Open(fs.getSectionFileName(id, section))
}

func (fs *Fs) CreateSection(id id.ID, section string) (io.WriteCloser, error) {
	err := checkSectionNameSanity(section)
	if err != nil {
		return nil, err
	}

	return os.Create(fs.getSectionFileName(id, section))
}

func (fs *Fs) DeleteSection(id id.ID, section string) error {
	err := checkSectionNameSanity(section)
	if err != nil {
		return err
	}

	return os.Remove(fs.getSectionFileName(id, section))
}

func (fs *Fs) loadRecords() error {
	entries, err := os.ReadDir(fs.basePath)
	if err != nil {
		return err
	}

	var recordFiles []string

	for _, e := range entries {
		if e.Type().IsDir() {
			return errors.New("garbage directory in fs root")
		}

		name := e.Name()

		if !onlyFileInFsRootPatternRegex.MatchString(name) {
			return fmt.Errorf("garbage file in fs root: %s", name)
		}

		if len(name) == 22 {
			recordFiles = append(recordFiles, name)
		} // else { TODO: file sections }
	}

	for _, recordName := range recordFiles {
		u, err := id.Parse(recordName)
		if err != nil {
			return err
		}

		f, err := os.Open(fs.path(recordName))
		if err != nil {
			return err
		}
		defer f.Close()

		rec := new(record)
		dec := json.NewDecoder(f)
		err = dec.Decode(rec)
		if err != nil {
			return fmt.Errorf("json decore err: %w", err)
		}

		fs.records[u] = rec
	}

	// TODO(prokop) load section file names
	return nil
}

func checkLoadedRecordsAreSane(map[id.ID]*record) error {
	// TODO(prokop)
	return nil
}

func NewFs(root id.ID, basePath string) (fs *Fs, err error) {
	fs = new(Fs)
	fs.basePath = basePath
	fs.root = root
	fs.records = make(map[id.ID]*record)

	err = fs.loadRecords()
	if err != nil {
		return
	}

	if _, c := fs.records[root]; !c {
		err = errors.New("the root ID not found in fs")
		return
	}

	return fs, checkLoadedRecordsAreSane(fs.records)
}

// function argument `dir` has to be checked by the caller. It is assumed that
// this dir already exists
// InitFsDir creates the following directory structure:
//
//	dir/
//	├── files/
//	│   └── WXC2BGKFiiDAjBWbf6wayV
//	└── users/
//	    ├── ...
//	    └── ...
//
// Used to setup a server in unittests.
func InitFsDir(dir string, users map[string][64]byte) (rootID id.ID, err error) {
	fsDir := filepath.Join(dir, "files")
	rootID = id.New()
	rootIDPath := filepath.Join(fsDir, rootID.String())
	usersDir := filepath.Join(dir, "users")

	if err = os.Mkdir(fsDir, 0750); err != nil {
		err = fmt.Errorf("mkdir: %w", err)
		return
	}

	if err = os.Mkdir(usersDir, 0750); err != nil {
		err = fmt.Errorf("mkdir: %w", err)
		return
	}

	f, err := os.Create(rootIDPath) // #nosec G304: the dir argument is trusted
	if err != nil {
		err = fmt.Errorf("create root id: %w", err)
		return
	}
	defer f.Close()

	if err = json.NewEncoder(f).Encode(record{IsDir: true}); err != nil {
		err = fmt.Errorf("encode root record: %w", err)
		return
	}

	for user, pwd := range users {
		userFilePath := filepath.Join(usersDir, user)
		err = os.WriteFile(userFilePath, pwd[:], 0600)
		if err != nil {
			err = fmt.Errorf("write user file: %w", err)
			return
		}
	}

	return
}
