package gitrepotest

import (
	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/gitrepo"
)

var _ gitrepo.GitRepo = (*MockGitRepo)(nil)

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
	ListDirFilesFn       func(clonePath, branch, dirPrefix string) ([]string, error)
}

func (m *MockGitRepo) Clone(url, clonePath string) error {
	if m.CloneFn == nil {
		panic("called not defined CloneFn")
	}
	return m.CloneFn(url, clonePath)
}

func (m *MockGitRepo) Fetch(clonePath, branch string) (string, error) {
	if m.FetchFn == nil {
		panic("called not defined FetchFn")
	}
	return m.FetchFn(clonePath, branch)
}

func (m *MockGitRepo) DiffSinceCommit(clonePath, branch, oldSHA string) ([]domain.FileDiff, error) {
	if m.DiffSinceCommitFn == nil {
		panic("called not defined DiffSinceCommitFn")
	}
	return m.DiffSinceCommitFn(clonePath, branch, oldSHA)
}

func (m *MockGitRepo) ReadFileAtRef(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
	if m.ReadFileAtRefFn == nil {
		panic("called not defined ReadFileAtRefFn")
	}
	return m.ReadFileAtRefFn(clonePath, branch, filePath, commitSHA)
}

func (m *MockGitRepo) FileVersion(clonePath, filePath string) (domain.MctVersion, error) {
	if m.FileVersionFn == nil {
		panic("called not defined FileVersionFn")
	}
	return m.FileVersionFn(clonePath, filePath)
}

func (m *MockGitRepo) RemoteHEAD(clonePath, branch string) (string, error) {
	if m.RemoteHEADFn == nil {
		panic("called not defined RemoteHEADFn")
	}
	return m.RemoteHEADFn(clonePath, branch)
}

func (m *MockGitRepo) ListFiles(clonePath, branch string) ([]string, error) {
	if m.ListFilesFn == nil {
		panic("called not defined ListFilesFn")
	}
	return m.ListFilesFn(clonePath, branch)
}

func (m *MockGitRepo) IsValidRepo(clonePath string) bool {
	if m.IsValidRepoFn == nil {
		panic("called not defined IsValidRepoFn")
	}
	return m.IsValidRepoFn(clonePath)
}

func (m *MockGitRepo) ValidateRemote(url string) error {
	if m.ValidateRemoteFn == nil {
		panic("called not defined ValidateRemoteFn")
	}
	return m.ValidateRemoteFn(url)
}

func (m *MockGitRepo) ReadGlobalDifftool() (string, error) {
	if m.ReadGlobalDifftoolFn == nil {
		panic("called not defined ReadGlobalDifftoolFn")
	}
	return m.ReadGlobalDifftoolFn()
}

func (m *MockGitRepo) ReadMarketFiles(clonePath, branch string) ([]gitrepo.MarketFile, error) {
	if m.ReadMarketFilesFn == nil {
		panic("called not defined ReadMarketFilesFn")
	}
	return m.ReadMarketFilesFn(clonePath, branch)
}

func (m *MockGitRepo) ListDirFiles(clonePath, branch, dirPrefix string) ([]string, error) {
	if m.ListDirFilesFn == nil {
		panic("called not defined ListDirFilesFn")
	}
	return m.ListDirFilesFn(clonePath, branch, dirPrefix)
}
