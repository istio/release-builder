// Copyright Istio Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
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

// VerboseCommand runs a command, outputing stderr and stdout
func VerboseCommand(name string, arg ...string) *exec.Cmd {
	log.Infof("Running command: %v %v in", name, strings.Join(arg, " "))
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

// CopyDirFiltered copies a directory, but only includes files that match given patterns
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

// CreateSha will create and write a sha256sum of a file
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
	log.Infof("Copying %v -> %v", src, dst)
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
	err := VerboseCommand("git", "clone", url, dest).Run()
	if err != nil {
		return err
	}
	cmd := VerboseCommand("git", "checkout", repo.Ref())
	cmd.Dir = dest
	return cmd.Run()
}
