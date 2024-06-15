package git

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"net/url"
	"os"
	"path"
	"strings"
)

const ClonePath = "/tmp/kaytu-gits"

type Service struct {
}

func New() *Service {
	return &Service{}
}

func (s *Service) Clone(gitURL, branch, user, pass string) error {
	_, err := git.PlainClone(s.GitFolder(gitURL), false, &git.CloneOptions{
		URL:           gitURL,
		ReferenceName: plumbing.ReferenceName(branch),
		Auth: &http.BasicAuth{
			Username: user,
			Password: pass,
		},
		Progress: os.Stdout,
	})
	if err != nil {
		if !strings.Contains(err.Error(), "repository already exists") {
			return err
		}
	}
	return nil
}

func (s *Service) GitFolder(gitURL string) string {
	gitPath, _ := urlToFolderPath(gitURL)
	return gitPath
}
func (s *Service) CleanUp() error {
	return os.RemoveAll(ClonePath)
}

func urlToFolderPath(urlStr string) (string, error) {
	urlStr = strings.TrimSuffix(urlStr, ".git")
	// Parse the URL.
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	// Sanitize the URL path to be used as a folder path.
	folderPath := strings.Trim(parsedURL.Path, "/")

	// Combine the sanitized path with the host to create a folder path.
	return path.Join(ClonePath, parsedURL.Host, folderPath), nil
}
