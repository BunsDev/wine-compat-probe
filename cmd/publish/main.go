// Command publish commits files to a branch through the GitHub GraphQL API
// (createCommitOnBranch). GitHub signs commits created this way, so a CI
// runner without a signing key can still produce verified commits.
// Missing files are skipped with a notice so partial result sets still land.
//
// Usage:
//
//	GH_TOKEN=... publish -repo owner/name -branch main -message TEXT [-body TEXT] path [path ...]
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	apiREST    = `https://api.github.com`
	apiGraphQL = `https://api.github.com/graphql`
)

const mutation = `
mutation($input: CreateCommitOnBranchInput!) {
  createCommitOnBranch(input: $input) {
    commit { oid url }
  }
}`

func main() {
	repo := flag.String(`repo`, os.Getenv(`GITHUB_REPOSITORY`), `owner/name`)
	branch := flag.String(`branch`, `main`, `branch to commit to`)
	message := flag.String(`message`, `results: publish`, `commit headline`)
	body := flag.String(`body`, ``, `commit body (optional)`)
	flag.Parse()
	token := firstEnv(`GH_TOKEN`, `GITHUB_TOKEN`)
	if token == `` {
		fatal(errors.New(`GH_TOKEN or GITHUB_TOKEN is required`))
	}
	if *repo == `` || !strings.Contains(*repo, `/`) {
		fatal(errors.New(`-repo owner/name is required`))
	}
	if flag.NArg() == 0 {
		fatal(errors.New(`at least one file path is required`))
	}
	client := &http.Client{Timeout: 60 * time.Second}
	head, err := headOID(client, token, *repo, *branch)
	if err != nil {
		fatal(err)
	}
	var additions []map[string]string
	for _, p := range flag.Args() {
		b, err := os.ReadFile(p)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				fmt.Fprintln(os.Stderr, `publish: skipping missing file`, p)
				continue
			}
			fatal(err)
		}
		add := map[string]string{}
		add[`path`] = p
		add[`contents`] = base64.StdEncoding.EncodeToString(b)
		additions = append(additions, add)
	}
	if len(additions) == 0 {
		fatal(errors.New(`nothing to commit: every listed file is missing`))
	}
	branchRef := map[string]string{}
	branchRef[`repositoryNameWithOwner`] = *repo
	branchRef[`branchName`] = *branch
	msg := map[string]string{}
	msg[`headline`] = *message
	if *body != `` {
		msg[`body`] = *body
	}
	changes := map[string]any{}
	changes[`additions`] = additions
	input := map[string]any{}
	input[`branch`] = branchRef
	input[`message`] = msg
	input[`expectedHeadOid`] = head
	input[`fileChanges`] = changes
	vars := map[string]any{}
	vars[`input`] = input
	payload := map[string]any{}
	payload[`query`] = mutation
	payload[`variables`] = vars
	resp, err := postJSON(client, token, apiGraphQL, payload)
	if err != nil {
		fatal(err)
	}
	var out struct {
		Data struct {
			CreateCommitOnBranch struct {
				Commit struct {
					OID string
					URL string
				}
			}
		}
		Errors []struct {
			Message string
		}
	}
	if err := json.Unmarshal(resp, &out); err != nil {
		fatal(fmt.Errorf(`decode response: %w`, err))
	}
	if len(out.Errors) > 0 {
		fatal(errors.New(`graphql: ` + out.Errors[0].Message))
	}
	fmt.Println(out.Data.CreateCommitOnBranch.Commit.OID, out.Data.CreateCommitOnBranch.Commit.URL)
}

func headOID(client *http.Client, token, repo, branch string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, apiREST+`/repos/`+repo+`/git/ref/heads/`+branch, nil)
	if err != nil {
		return ``, err
	}
	req.Header.Set(`Authorization`, `Bearer `+token)
	req.Header.Set(`Accept`, `application/vnd.github+json`)
	resp, err := client.Do(req)
	if err != nil {
		return ``, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return ``, fmt.Errorf(`ref lookup: HTTP %d: %s`, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var ref struct {
		Object struct {
			SHA string
		}
	}
	if err := json.Unmarshal(b, &ref); err != nil {
		return ``, err
	}
	if ref.Object.SHA == `` {
		return ``, errors.New(`ref lookup: empty sha`)
	}
	return ref.Object.SHA, nil
}

func postJSON(client *http.Client, token, url string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set(`Authorization`, `Bearer `+token)
	req.Header.Set(`Content-Type`, `application/json`)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(`graphql: HTTP %d: %s`, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return b, nil
}

func firstEnv(names ...string) string {
	for _, n := range names {
		if v := os.Getenv(n); v != `` {
			return v
		}
	}
	return ``
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, `publish:`, err)
	os.Exit(1)
}
