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
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rogpeppe/go-internal/modfile"

	"istio.io/pkg/log"
	"istio.io/release-builder/pkg/model"
)

// VerboseCommand runs a command, outputting stderr and stdout
func VerboseCommand(name string, arg ...string) *exec.Cmd {
	log.Infof("Running command: %v %v", name, strings.Join(arg, " "))
	cmd := exec.Command(name, arg...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd
}

// RunWithOutput runs a command, outputting stderr and stdout, and returning the command's stdout
func RunWithOutput(name string, arg ...string) (string, error) {
	var outBuffer bytes.Buffer
	var errBuffer bytes.Buffer
	cmd := VerboseCommand(name, arg...)
	cmd.Stdout = io.MultiWriter(os.Stdout, &outBuffer)
	cmd.Stderr = io.MultiWriter(os.Stderr, &errBuffer)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("running command %s failed: %s: %s",
			strings.Join(arg, " "), err.Error(), errBuffer.String())
	}
	return outBuffer.String(), nil
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

// CopyFilesToDir copies all files in one directory to another
func CopyFilesToDir(src, dst string) error {
	if err := VerboseCommand("mkdir", "-p", path.Join(dst, "..")).Run(); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}
	dir, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}
	for _, i := range dir {
		if err := CopyFile(filepath.Join(src, i.Name()), filepath.Join(dst, i.Name())); err != nil {
			return fmt.Errorf("failed to copy: %v", err)
		}
	}
	return nil
}

// FileExists checks if a file exists
func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
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

func Clone(repo string, dep model.Dependency, dest string) error {
	if dep.LocalPath != "" {
		return CopyDir(dep.LocalPath, dest)
	}
	if dep.Auto != "" {
		// In Auto mode the dependency will be update to have the correct sha applied
		if err := FetchAuto(repo, &dep, dest); err != nil {
			return err
		}
	}
	args := []string{"clone", dep.Git, dest}
	// As an optimization, if we are cloning a branch just shallow clone
	if dep.Branch != "" {
		args = append(args, "-b", dep.Branch, "--depth=1")
	}
	// We must be fetching from git
	err := VerboseCommand("git", args...).Run()
	if err != nil {
		return err
	}

	cmd := VerboseCommand("git", "checkout", dep.Ref())
	cmd.Dir = dest
	return cmd.Run()
}

// FetchAuto looks up the SHA to use for the dependency from istio/istio
func FetchAuto(repo string, dep *model.Dependency, dest string) error {
	if dep.Auto == model.Deps {
		return fetchAutoDeps(repo, dep, dest)
	} else if dep.Auto == model.Modules {
		return fetchAutoModules(repo, dep, dest)
	} else if dep.Auto == model.ProxyWorkspace {
		return fetchAutoProxyWorkspace(dep, dest)
	}
	return fmt.Errorf("unknown auto dependency: %v", dep.Auto)
}

func fetchAutoModules(repo string, dep *model.Dependency, dest string) error {
	modFile, err := ioutil.ReadFile(path.Join(dest, "../istio/go.mod"))
	if err != nil {
		return err
	}
	mod, err := modfile.Parse("", modFile, nil)
	if err != nil {
		return err
	}
	for _, r := range mod.Require {
		if r.Mod.Path == "istio.io/"+repo {
			ver := r.Mod.Version
			if len(strings.Split(ver, "-")) == 3 {
				// We are dealing with a pseudo version
				ver = strings.Split(ver, "-")[2]
			}
			dep.Sha = ver
			return nil
		}
	}

	return fmt.Errorf("failed to automatically resolve source for %v", repo)
}

func fetchAutoDeps(repo string, dep *model.Dependency, dest string) error {
	depsFile, err := ioutil.ReadFile(path.Join(dest, "../istio/istio.deps"))
	if err != nil {
		return err
	}
	deps := make([]model.IstioDep, 0)
	if err := json.Unmarshal(depsFile, &deps); err != nil {
		return err
	}
	var sha string
	for _, d := range deps {
		if d.RepoName == repo {
			sha = d.LastStableSHA
		}
	}
	if sha == "" {
		return fmt.Errorf("failed to automatically resolve source for %v", repo)
	}
	dep.Sha = sha
	return nil
}

func fetchAutoProxyWorkspace(dep *model.Dependency, dest string) error {
	wsFile, err := ioutil.ReadFile(path.Join(dest, "../proxy/WORKSPACE"))
	if err != nil {
		return err
	}
	// ENVOY_SHA is declared in proxy workspace file.
	esReg := regexp.MustCompile("ENVOY_SHA = \"([a-z0-9]{40})\"")
	var sha string
	if found := esReg.FindStringSubmatch(string(wsFile)); len(found) == 2 {
		sha = found[1]
	}

	if sha == "" {
		return errors.New("failed to automatically resolve source for envoy")
	}
	dep.Sha = sha
	return nil
}

func ZipFolder(source, target string) error {
	zipfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer zipfile.Close()

	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	info, err := os.Stat(source)
	if err != nil {
		return nil
	}

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(source)
	}

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		if baseDir != "" {
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
		}

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	})
}
