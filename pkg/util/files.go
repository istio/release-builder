package util

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/howardjohn/istio-release/pkg/model"

	"istio.io/pkg/log"
)

func VerboseCommand(name string, arg ...string) *exec.Cmd {
	log.Infof("Running command %v %v", name, strings.Join(arg, " "))
	cmd := exec.Command(name, arg...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd
}

func CopyDir(src, dst string) error {
	if err := VerboseCommand("mkdir", "-p", path.Join(dst, "..")).Run(); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}
	if err := VerboseCommand("cp", "-r", src, dst).Run(); err != nil {
		return fmt.Errorf("failed to copy: %v", err)
	}
	return nil
}

func CopyDirFiltered(src, dst string, include []string) error {
	if err := CopyDir(src, dst); err != nil {
		return err
	}
	if err := filepath.Walk(dst, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		fname := filepath.Base(path)
		for _, pattern := range include {
			if matched, _ := filepath.Match(pattern, fname); matched {
				// It matches one of the patterns, so stop early
				return nil
			}
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("failed to remove filted file %v: %v", path, err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to filter: %v", err)
	}
	return nil
}

func CreateSha(src string) error {
	b, err := ioutil.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read file %v: %v", src, err)
	}
	sha := sha256.Sum256(b)
	shaFile := fmt.Sprintf("%x %s\n", sha, path.Base(src))
	if err := ioutil.WriteFile(src+".sha256", []byte(shaFile), 0644); err != nil {
		return fmt.Errorf("failed to write sha256 to %v: %v", src, err)
	}
	return nil
}

func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open file %v to copy: %v", src, err)
	}
	defer in.Close()

	if err := os.MkdirAll(path.Join(dst, ".."), 0750); err != nil {
		return fmt.Errorf("failed to make destination directory %v: %v", dst, err)
	}
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create file %v to copy to: %v", dst, err)
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return fmt.Errorf("failed to copy %v to %v: %v", src, dst, err)
	}

	return nil
}

func Clone(repo model.Dependency, dest string) error {
	url := fmt.Sprintf("https://github.com/%s/%s", repo.Org, repo.Repo)
	err := exec.Command("git", "clone", url, dest).Run()
	if err != nil {
		return err
	}
	cmd := exec.Command("git", "checkout", repo.Ref())
	cmd.Dir = dest
	return cmd.Run()
}

func Download(url string, dest string) error {
	// dirty hack
	command := fmt.Sprintf("curl -sL %s | tar xvfz - -C %s", url, dest)
	cmd := exec.Command("sh", "-c", command)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if stderr.String() != "" {
		log.Warnf("Error downloading %v: %v", url, stderr.String())
	}
	return err
}
