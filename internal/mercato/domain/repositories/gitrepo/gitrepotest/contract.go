package gitrepotest

import (
	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/gitrepo"
)

type MockGitRepo struct {
	CloneFn              func(url, clonePath string) error
	FetchFn              func(clonePath, branch string) (string, error)
	DiffSinceCommitFn    func(clonePath, branch, oldSHA string) ([]domain.FileDiff, error)
	ReadFileAtRefFn      func(clonePath, branch, filePath, commitSHA string) ([]byte, error)
	FileVersionFn        func(clonePath, filePath string) (domain.MctVersion, error)
	RemoteHEADFn         func(clonePath, branch string) (string, error)
	ListFilesFn          func(clonePath, branch string) ([]string, error)
	IsValidRepoFn        func(clonePath string) bool
	ValidateRemoteFn     func(url string) error
	ReadGlobalDifftoolFn func() (string, error)
	ReadMarketFilesFn    func(clonePath, branch string) ([]gitrepo.MarketFile, error)
}

func (m *MockGitRepo) Clone(url, clonePath string) error {
	if m.CloneFn != nil {
		return m.CloneFn(url, clonePath)
	}
	return nil
}

func (m *MockGitRepo) Fetch(clonePath, branch string) (string, error) {
	if m.FetchFn != nil {
		return m.FetchFn(clonePath, branch)
	}
	return "", nil
}

func (m *MockGitRepo) DiffSinceCommit(clonePath, branch, oldSHA string) ([]domain.FileDiff, error) {
	if m.DiffSinceCommitFn != nil {
		return m.DiffSinceCommitFn(clonePath, branch, oldSHA)
	}
	return nil, nil
}

func (m *MockGitRepo) ReadFileAtRef(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
	if m.ReadFileAtRefFn != nil {
		return m.ReadFileAtRefFn(clonePath, branch, filePath, commitSHA)
	}
	return nil, nil
}

func (m *MockGitRepo) FileVersion(clonePath, filePath string) (domain.MctVersion, error) {
	if m.FileVersionFn != nil {
		return m.FileVersionFn(clonePath, filePath)
	}
	return "", nil
}

func (m *MockGitRepo) RemoteHEAD(clonePath, branch string) (string, error) {
	if m.RemoteHEADFn != nil {
		return m.RemoteHEADFn(clonePath, branch)
	}
	return "", nil
}

func (m *MockGitRepo) ListFiles(clonePath, branch string) ([]string, error) {
	if m.ListFilesFn != nil {
		return m.ListFilesFn(clonePath, branch)
	}
	return nil, nil
}

func (m *MockGitRepo) IsValidRepo(clonePath string) bool {
	if m.IsValidRepoFn != nil {
		return m.IsValidRepoFn(clonePath)
	}
	return false
}

func (m *MockGitRepo) ValidateRemote(url string) error {
	if m.ValidateRemoteFn != nil {
		return m.ValidateRemoteFn(url)
	}
	return nil
}

func (m *MockGitRepo) ReadGlobalDifftool() (string, error) {
	if m.ReadGlobalDifftoolFn != nil {
		return m.ReadGlobalDifftoolFn()
	}
	return "", nil
}

func (m *MockGitRepo) ReadMarketFiles(clonePath, branch string) ([]gitrepo.MarketFile, error) {
	if m.ReadMarketFilesFn != nil {
		return m.ReadMarketFilesFn(clonePath, branch)
	}
	return nil, nil
}
