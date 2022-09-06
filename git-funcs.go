package ugotsrvd

// go-git helper functions

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// Pull changes from a remote repository
func gitPull(path string) {
	// CheckArgs("<path>")
	// path := os.Args[1]

	// We instantiate a new repository targeting the given path (the .git folder)
	r, err := git.PlainOpen(path)
	CheckIfError(err)

	// Get the working directory for the repository
	w, err := r.Worktree()
	CheckIfError(err)

	// Pull the latest changes from the origin remote and merge into the current branch
	Info("git pull origin")
	err = w.Pull(&git.PullOptions{RemoteName: "origin"})
	CheckIfError(err)

	// Print the latest commit that was just pulled
	ref, err := r.Head()
	CheckIfError(err)
	commit, err := r.CommitObject(ref.Hash())
	CheckIfError(err)

	fmt.Println(commit)
}

func gitPush(path, username, token, revision string) string {
	// CheckArgs("<repository-path>")
	// path := os.Args[1]

	var auth = &http.BasicAuth{
		Username: username,
		Password: token,
	}

	r, err := git.PlainOpen(path)
	CheckIfError(err)

	// Resolve revision into a sha1 commit, only some revisions are resolved
	// look at the doc to get more details
	Info("git rev-parse %s", revision)
	h, err2 := r.ResolveRevision(plumbing.Revision(revision))
	CheckIfError(err2)
	commitSHA := h.String()
	log.Println("commit SHA:", commitSHA)

	Info("git push")
	// push using default options
	// err = r.Push(&git.PushOptions{})
	err = r.Push(&git.PushOptions{
		Auth:     auth,
		Progress: os.Stdout,
	})
	CheckIfError(err)
	return commitSHA
}

// // An example of how to create and remove branches or any other kind of reference.
// func gitBranch(url, directory, branch string) {
// 	// CheckArgs("<url>", "<directory>")
// 	// url, directory := os.Args[1], os.Args[2]

// 	// // Clone the given repository to the given directory
// 	// Info("git clone %s %s", url, directory)
// 	// r, err := git.PlainClone(directory, false, &git.CloneOptions{
// 	// 	URL: url,
// 	// })
// 	// CheckIfError(err)

// 	r, err := git.PlainOpen(directory)
// 	CheckIfError(err)

// 	// Create a new branch to the current HEAD
// 	// Info("git branch my-branch")
// 	Info("git branch " + branch)

// 	headRef, err := r.Head()
// 	CheckIfError(err)

// 	// Create a new plumbing.HashReference object with the name of the branch
// 	// and the hash from the HEAD. The reference name should be a full reference
// 	// name and not an abbreviated one, as is used on the git cli.
// 	//
// 	// For tags we should use `refs/tags/%s` instead of `refs/heads/%s` used
// 	// for branches.
// 	// ref := plumbing.NewHashReference("refs/heads/my-branch", headRef.Hash())
// 	// refVal := fmt.Sprintf("refs/heads/%v", branch)
// 	ref := plumbing.NewHashReference("refs/heads/"+branch, headRef.Hash())

// 	// The created reference is saved in the storage.
// 	err = r.Storer.SetReference(ref)
// 	CheckIfError(err)

// 	// // Or deleted from it.
// 	// // Info("git branch -D my-branch")
// 	// Info("git branch -D " + branch)
// 	// err = r.Storer.RemoveReference(ref.Name())
// 	// CheckIfError(err)
// }

func gitClone(url, directory, username, token string) {
	// CheckArgs("<url>", "<directory>", "<github_access_token>")
	// url, directory, token := os.Args[1], os.Args[2], os.Args[3]

	// Clone the given repository to the given directory
	Info("git clone %s %s", url, directory)

	r, err := git.PlainClone(directory, false, &git.CloneOptions{
		// The intended use of a GitHub personal access token is in replace of your password
		// because access tokens can easily be revoked.
		// https://help.github.com/articles/creating-a-personal-access-token-for-the-command-line/
		Auth: &http.BasicAuth{
			// Username: "abc123", // yes, this can be anything except an empty string
			Username: username, // yes, this can be anything except an empty string
			Password: token,
		},
		URL:      url,
		Progress: os.Stdout,
	})
	CheckIfError(err)

	// ... retrieving the branch being pointed by HEAD
	ref, err := r.Head()
	CheckIfError(err)
	// ... retrieving the commit object
	commit, err := r.CommitObject(ref.Hash())
	CheckIfError(err)

	log.Println(commit)
}

// // Clone git repo
// func gitClone(url, directory string) {
// 	// CheckArgs("<url>", "<directory>")
// 	// url := os.Args[1]
// 	// directory := os.Args[2]

// 	// Clone the given repository to the given directory
// 	Info("git clone %s %s --recursive", url, directory)

// 	r, err := git.PlainClone(directory, false, &git.CloneOptions{
// 		URL:               url,
// 		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
// 	})

// 	CheckIfError(err)

// 	// ... retrieving the branch being pointed by HEAD
// 	ref, err := r.Head()
// 	CheckIfError(err)
// 	// ... retrieving the commit object
// 	commit, err := r.CommitObject(ref.Hash())
// 	CheckIfError(err)

// 	log.Println(commit)
// }

// Git Commit
func gitCommit(gitDirectory, message string, files []string) {

	// Opens an already existing repository.
	r, err := git.PlainOpen(gitDirectory)
	CheckIfError(err)

	w, err := r.Worktree()
	CheckIfError(err)

	// Adds the new file to the staging area.
	// Info("git add example-git-file")
	// Info("git add " + chartname)
	// _, err = w.Add(chartname)
	for _, f := range files {
		Info("git add " + f)
		_, err = w.Add(f)
		CheckIfError(err)
	}

	// We can verify the current status of the worktree using the method Status.
	Info("git status --porcelain")
	status, err := w.Status()
	CheckIfError(err)

	log.Println(status)
	// Commits the current staging area to the repository, with the new file
	// just created. We should provide the object.Signature of Author of the
	// commit Since version 5.0.1, we can omit the Author signature, being read
	// from the git config files.
	// Info("git commit -m \"Example go-git commit\"")
	Info("git commit -m \"Example go-git commit\"")
	commit, err := w.Commit("Example go-git commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Ron Compos",
			Email: "rcompos@gmail.com",
			When:  time.Now(),
		},
	})
	CheckIfError(err)

	// Prints the current HEAD to verify that all worked well.
	Info("git show -s")
	obj, err := r.CommitObject(commit)
	CheckIfError(err)

	log.Println(obj)
	log.Println(message)

}
