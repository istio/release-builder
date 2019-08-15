package util

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

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

func Download(url string, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	return ExtractTarGz(resp.Body, dest)
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
