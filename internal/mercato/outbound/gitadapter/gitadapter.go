package gitadapter

import (
	"fmt"
	"os"
	"strings"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/repositories/gitrepo"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	"github.com/go-git/go-git/v5/utils/merkletrie"
)

var _ gitrepo.GitRepo = (*Adapter)(nil)

type Adapter struct {
	sshEnabled bool
}

func New(opts ...Option) *Adapter {
	a := &Adapter{}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

type Option func(*Adapter)

func WithSSHEnabled(enabled bool) Option {
	return func(a *Adapter) {
		a.sshEnabled = enabled
	}
}

func (a *Adapter) Clone(url, clonePath string) error {
	if isSSHURL(url) && !a.sshEnabled {
		return domain.ErrSSHDisabled
	}
	_, err := git.PlainClone(clonePath, false, &git.CloneOptions{
		URL:      url,
		Auth:     resolveAuth(url),
		Depth:    1,
		Progress: os.Stdout,
	})
	return err
}

func (a *Adapter) Fetch(clonePath, branch string) (string, error) {
	repo, err := git.PlainOpen(clonePath)
	if err != nil {
		return "", fmt.Errorf("open repo: %w", err)
	}

	if !a.sshEnabled {
		if remote, rErr := repo.Remote("origin"); rErr == nil && remote != nil {
			if urls := remote.Config().URLs; len(urls) > 0 && isSSHURL(urls[0]) {
				return "", domain.ErrSSHDisabled
			}
		}
	}

	err = repo.Fetch(&git.FetchOptions{
		Auth:  resolveAuthFromRepo(repo),
		Depth: 1,
		Prune: true,
		Force: false,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return "", fmt.Errorf("fetch: %w", err)
	}

	ref, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", branch), true)
	if err != nil {
		return "", fmt.Errorf("resolve remote ref: %w", err)
	}

	return ref.Hash().String(), nil
}

func (a *Adapter) DiffSinceCommit(clonePath, branch, oldSHA string) ([]domain.FileDiff, error) {
	repo, err := git.PlainOpen(clonePath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	oldHash := plumbing.NewHash(oldSHA)
	oldCommit, err := repo.CommitObject(oldHash)
	if err != nil {
		return nil, fmt.Errorf("resolve old commit: %w", err)
	}

	remoteRef, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", branch), true)
	if err != nil {
		return nil, fmt.Errorf("resolve remote ref: %w", err)
	}

	newCommit, err := repo.CommitObject(remoteRef.Hash())
	if err != nil {
		return nil, fmt.Errorf("resolve new commit: %w", err)
	}

	oldTree, err := oldCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get old tree: %w", err)
	}

	newTree, err := newCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get new tree: %w", err)
	}

	changes, err := oldTree.Diff(newTree)
	if err != nil {
		return nil, fmt.Errorf("diff trees: %w", err)
	}

	var diffs []domain.FileDiff
	for _, change := range changes {
		action, err := change.Action()
		if err != nil {
			return nil, fmt.Errorf("get change action: %w", err)
		}

		var domainAction domain.DiffAction
		switch action {
		case merkletrie.Insert:
			domainAction = domain.DiffInsert
		case merkletrie.Modify:
			domainAction = domain.DiffModify
		case merkletrie.Delete:
			domainAction = domain.DiffDelete
		default:
			continue
		}

		path := change.To.Name
		if path == "" {
			path = change.From.Name
		}

		if !isAgentOrSkillPath(path) {
			continue
		}

		diffs = append(diffs, domain.FileDiff{
			Action: domainAction,
			From:   change.From.Name,
			To:     change.To.Name,
		})
	}

	return diffs, nil
}

func (a *Adapter) ReadFileAtRef(clonePath, branch, filePath, commitSHA string) ([]byte, error) {
	repo, err := git.PlainOpen(clonePath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	var hash plumbing.Hash
	if commitSHA == "HEAD" || commitSHA == "" {
		ref, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", branch), true)
		if err != nil {
			return nil, fmt.Errorf("resolve remote ref: %w", err)
		}
		hash = ref.Hash()
	} else {
		hash = plumbing.NewHash(commitSHA)
	}

	commit, err := repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("resolve commit: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}

	file, err := tree.File(filePath)
	if err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}

	contents, err := file.Contents()
	if err != nil {
		return nil, fmt.Errorf("read file contents: %w", err)
	}

	return []byte(contents), nil
}

func (a *Adapter) FileVersion(clonePath, filePath string) (domain.MctVersion, error) {
	repo, err := git.PlainOpen(clonePath)
	if err != nil {
		return "", fmt.Errorf("open repo: %w", err)
	}

	logs, err := repo.Log(&git.LogOptions{
		FileName: &filePath,
		Order:    git.LogOrderCommitterTime,
	})
	if err != nil {
		return "", fmt.Errorf("git log: %w", err)
	}

	commit, err := logs.Next()
	if err != nil {
		return "", fmt.Errorf("get first commit: %w", err)
	}

	sha := commit.Hash.String()
	version := sha[:7] + "\u00b7" + commit.Author.When.Format("2006-01-02")

	return domain.MctVersion(version), nil
}

func (a *Adapter) RemoteHEAD(clonePath, branch string) (string, error) {
	repo, err := git.PlainOpen(clonePath)
	if err != nil {
		return "", fmt.Errorf("open repo: %w", err)
	}

	ref, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", branch), true)
	if err != nil {
		return "", fmt.Errorf("resolve remote ref: %w", err)
	}

	return ref.Hash().String(), nil
}

func (a *Adapter) ListFiles(clonePath, branch string) ([]string, error) {
	repo, err := git.PlainOpen(clonePath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	ref, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", branch), true)
	if err != nil {
		return nil, fmt.Errorf("resolve remote ref: %w", err)
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("resolve commit: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}

	var paths []string
	err = tree.Files().ForEach(func(f *object.File) error {
		if strings.HasSuffix(f.Name, ".md") && (isAgentOrSkillPath(f.Name) || isReadmePath(f.Name)) {
			paths = append(paths, f.Name)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk files: %w", err)
	}

	return paths, nil
}

func (a *Adapter) ReadMarketFiles(clonePath, branch string) ([]gitrepo.MarketFile, error) {
	repo, err := git.PlainOpen(clonePath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	ref, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", branch), true)
	if err != nil {
		return nil, fmt.Errorf("resolve remote ref: %w", err)
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("resolve commit: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}

	// Collect all matching files and read their content in one tree walk.
	var paths []string
	contentByPath := make(map[string][]byte)
	err = tree.Files().ForEach(func(f *object.File) error {
		if !strings.HasSuffix(f.Name, ".md") {
			return nil
		}
		if !isAgentOrSkillPath(f.Name) && !isReadmePath(f.Name) {
			return nil
		}
		paths = append(paths, f.Name)
		c, err := f.Contents()
		if err != nil {
			return nil
		}
		contentByPath[f.Name] = []byte(c)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk files: %w", err)
	}

	versionByPath := batchFileVersions(repo, paths)

	result := make([]gitrepo.MarketFile, 0, len(paths))
	for _, p := range paths {
		result = append(result, gitrepo.MarketFile{
			Path:    p,
			Content: contentByPath[p],
			Version: versionByPath[p],
		})
	}
	return result, nil
}

func (a *Adapter) ListDirFiles(clonePath, branch, dirPrefix string) ([]string, error) {
	repo, err := git.PlainOpen(clonePath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	ref, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", branch), true)
	if err != nil {
		return nil, fmt.Errorf("resolve remote ref: %w", err)
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("resolve commit: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}

	prefix := dirPrefix
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	var paths []string
	err = tree.Files().ForEach(func(f *object.File) error {
		if strings.HasPrefix(f.Name, prefix) {
			paths = append(paths, f.Name)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk files: %w", err)
	}
	return paths, nil
}

var errStopIter = fmt.Errorf("stop")

// batchFileVersions walks the commit log once and returns the last-modified
// version string for each file path.
func batchFileVersions(repo *git.Repository, paths []string) map[string]domain.MctVersion {
	versions := make(map[string]domain.MctVersion, len(paths))
	remaining := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		remaining[p] = struct{}{}
	}

	logs, err := repo.Log(&git.LogOptions{Order: git.LogOrderCommitterTime})
	if err != nil {
		return versions
	}

	var prevCommit *object.Commit
	_ = logs.ForEach(func(c *object.Commit) error {
		if len(remaining) == 0 {
			return errStopIter
		}

		if prevCommit == nil {
			prevCommit = c
			return nil
		}

		// Compare previous (newer) commit tree with current (older) commit tree.
		prevTree, err := prevCommit.Tree()
		if err != nil {
			prevCommit = c
			return nil
		}
		curTree, err := c.Tree()
		if err != nil {
			prevCommit = c
			return nil
		}

		changes, err := prevTree.Diff(curTree)
		if err != nil {
			prevCommit = c
			return nil
		}

		for _, change := range changes {
			// Check both From and To names for the changed file.
			for _, name := range []string{change.From.Name, change.To.Name} {
				if name == "" {
					continue
				}
				if _, need := remaining[name]; !need {
					continue
				}
				// prevCommit is the newer commit where this file last changed.
				sha := prevCommit.Hash.String()
				ver := sha[:7] + "\u00b7" + prevCommit.Author.When.Format("2006-01-02")
				versions[name] = domain.MctVersion(ver)
				delete(remaining, name)
			}
		}

		prevCommit = c
		return nil
	})

	// Files still remaining were only touched in the very first commit.
	if prevCommit != nil {
		for p := range remaining {
			sha := prevCommit.Hash.String()
			ver := sha[:7] + "\u00b7" + prevCommit.Author.When.Format("2006-01-02")
			versions[p] = domain.MctVersion(ver)
		}
	}

	return versions
}

func (a *Adapter) IsValidRepo(clonePath string) bool {
	_, err := git.PlainOpen(clonePath)
	return err == nil
}

func (a *Adapter) ValidateRemote(url string) error {
	if isSSHURL(url) && !a.sshEnabled {
		return domain.ErrSSHDisabled
	}
	auth := resolveAuth(url)
	ep, err := transport.NewEndpoint(url)
	if err != nil {
		return err
	}
	cli, err := client.NewClient(ep)
	if err != nil {
		return err
	}
	sess, err := cli.NewUploadPackSession(ep, auth)
	if err != nil {
		return err
	}
	defer sess.Close()
	_, err = sess.AdvertisedReferences()
	return err
}

func (a *Adapter) ReadGlobalDifftool() (string, error) {
	cfg, err := config.LoadConfig(config.GlobalScope)
	if err != nil {
		return "", nil
	}
	tool := cfg.Raw.Section("diff").Option("tool")
	return tool, nil
}
