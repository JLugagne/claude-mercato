package installdbtest

import (
	"encoding/json"
	"path/filepath"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain"
	"github.com/JLugagne/agents-mercato/internal/mercato/domain/repositories/installdb"
)

var _ installdb.InstallDB = (*MockInstallDB)(nil)

type MockInstallDB struct {
	LoadFn    func(cacheDir string) (domain.InstallDatabase, error)
	SaveFn    func(cacheDir string, db domain.InstallDatabase) error
	MarshalFn func(db domain.InstallDatabase) ([]byte, error)
	PathFn    func(cacheDir string) string
	LockFn    func(cacheDir string) error
	UnlockFn  func(cacheDir string) error
}

func (m *MockInstallDB) Load(cacheDir string) (domain.InstallDatabase, error) {
	if m.LoadFn == nil {
		panic("called not defined LoadFn")
	}
	return m.LoadFn(cacheDir)
}

func (m *MockInstallDB) Save(cacheDir string, db domain.InstallDatabase) error {
	if m.SaveFn == nil {
		panic("called not defined SaveFn")
	}
	return m.SaveFn(cacheDir, db)
}

func (m *MockInstallDB) Marshal(db domain.InstallDatabase) ([]byte, error) {
	if m.MarshalFn != nil {
		return m.MarshalFn(db)
	}
	return json.Marshal(db)
}

func (m *MockInstallDB) Path(cacheDir string) string {
	if m.PathFn != nil {
		return m.PathFn(cacheDir)
	}
	return filepath.Join(cacheDir, "installed.json")
}

func (m *MockInstallDB) Lock(cacheDir string) error {
	if m.LockFn == nil {
		panic("called not defined LockFn")
	}
	return m.LockFn(cacheDir)
}

func (m *MockInstallDB) Unlock(cacheDir string) error {
	if m.UnlockFn == nil {
		panic("called not defined UnlockFn")
	}
	return m.UnlockFn(cacheDir)
}
