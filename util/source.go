package util

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/howardjohn/istio-release/pkg"
	"github.com/pkg/errors"

	"istio.io/pkg/log"
)

func CopyDir(src, dst string) error {
	if err := exec.Command("mkdir", "-p", path.Join(dst, "..")).Run(); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}
	if err := exec.Command("cp", "-r", src, dst).Run(); err != nil {
		return fmt.Errorf("failed to copy: %v", err)
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

func Clone(repo pkg.Dependency, dest string) error {
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
