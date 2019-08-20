package util

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/howardjohn/istio-release/pkg/model"
	"github.com/pkg/errors"

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
	if err := ioutil.WriteFile(src+".sha256", sha[:], 0644); err != nil {
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
func DownloadFile(filepath string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
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

// ExtractTarGz extracts a .tar.gz file into current dir.
func ExtractTarGz(gzippedStream io.Reader, dir string) error {
	uncompressedStream, err := gzip.NewReader(gzippedStream)
	if err != nil {
		return errors.Wrap(err, "Fail to uncompress")
	}
	tarReader := tar.NewReader(uncompressedStream)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return errors.Wrap(err, "ExtractTarGz: Next() failed")
		}

		rel := filepath.FromSlash(header.Name)
		abs := filepath.Join(dir, rel)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(abs, 0755); err != nil {
				return errors.Wrap(err, "ExtractTarGz: Mkdir() failed")
			}
		case tar.TypeReg:
			outFile, err := os.Create(abs)
			if err != nil {
				return errors.Wrap(err, "ExtractTarGz: Create() failed")
			}
			defer func() { _ = outFile.Close() }()
			if _, err := io.Copy(outFile, tarReader); err != nil {
				return errors.Wrap(err, "ExtractTarGz: Copy() failed")
			}
		case tar.TypeXGlobalHeader:
			// ignore the pax global header from git generated tarballs
			continue
		default:
			return fmt.Errorf("unknown type: %s in %s",
				string(header.Typeflag), header.Name)
		}
	}
	return nil
}
