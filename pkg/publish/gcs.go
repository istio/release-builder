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

package publish

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/googleapi"

	"istio.io/pkg/log"
	"istio.io/release-builder/pkg/model"
)

// GcsArchive publishes the final release archive to the given GCS bucket
func GcsArchive(manifest model.Manifest, bucket string, aliases []string) error {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		// TODO: Handle error.
		return err
	}

	// Allow the caller to pass a reference like bucket/folder/subfolder, but split this to
	// bucket, and folder/subfolder prefix
	splitbucket := strings.SplitN(bucket, "/", 2)
	bucketName := splitbucket[0]
	objectPrefix := ""
	if len(splitbucket) > 1 {
		objectPrefix = splitbucket[1]
	}
	bkt := client.Bucket(bucketName)
	if err := filepath.Walk(manifest.Directory, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		objName := path.Join(objectPrefix, manifest.Version, strings.TrimPrefix(p, manifest.Directory))
		obj := bkt.Object(objName)
		w := obj.NewWriter(ctx)
		f, err := os.Open(p)
		if err != nil {
			return fmt.Errorf("failed to open %v: %v", p, err)
		}
		r := bufio.NewReader(f)
		if _, err := io.Copy(w, r); err != nil {
			return fmt.Errorf("failed writing %v: %v", p, err)
		}
		if err := w.Close(); err != nil {
			return fmt.Errorf("failed to close bucket: %v", err)
		}
		log.Infof("Wrote %v to gs://%s/%s", p, bucketName, objName)
		return nil
	}); err != nil {
		return fmt.Errorf("failed to walk directory: %v", err)
	}

	// Add alias objects. These are basically symlinks/tags for GCS, pointing to the latest version
	for _, alias := range aliases {
		w := bkt.Object(path.Join(objectPrefix, alias)).NewWriter(ctx)
		if _, err := w.Write([]byte(manifest.Version)); err != nil {
			return fmt.Errorf("failed to write alias %v: %v", alias, err)
		}
		if err := w.Close(); err != nil {
			return fmt.Errorf("failed to close bucket: %v", err)
		}
		log.Infof("Wrote %v to gs://%s/%s", alias, bucketName, path.Join(objectPrefix, alias))
	}

	return nil
}

func FetchObject(bkt *storage.BucketHandle, objectPrefix string, filename string) ([]byte, error) {
	objName := filepath.Join(objectPrefix, filename)
	obj := bkt.Object(objName)
	r, err := obj.NewReader(context.Background())
	if err != nil {
		return nil, err
	}
	c, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// MutateObject allows pulling a file from GCS, mutating it, then pushing it back up. This adds checks
// to ensure that if the file is mutated in the meantime, the process is repeated.
func MutateObject(outDir string, bkt *storage.BucketHandle, objectPrefix string, filename string, f func() error) error {
	for i := 0; i < 10; i++ {
		err := mutateObjectInner(outDir, bkt, objectPrefix, filename, f)
		if err == ErrIndexOutOfDate {
			log.Warnf("Write conflict, trying again")
			continue
		}
		return err
	}
	return fmt.Errorf("max conflicts attempted")
}

func mutateObjectInner(outDir string, bkt *storage.BucketHandle, objectPrefix string, filename string, f func() error) error {
	objName := filepath.Join(objectPrefix, filename)
	outFile := filepath.Join(outDir, filename)
	obj := bkt.Object(objName)
	attr, err := obj.Attrs(context.Background())
	if err != nil {
		if err == storage.ErrObjectNotExist {
			// Missing is fine
			log.Warnf("existing file %v does not exist", filename)
		} else {
			return fmt.Errorf("failed to fetch attributes: %v", err)
		}
	}
	generation := int64(0)
	if attr != nil {
		generation = attr.Generation
		log.Infof("Object %v currently has generation %d", objName, generation)

		r, err := obj.NewReader(context.Background())
		if err != nil {
			if err == storage.ErrObjectNotExist {
				// This may be fine if we are publishing for the first time. Helm will allow us to `--merge non-existing-file.yaml`.
				log.Warnf("existing file %v does not exist", filename)
				return nil
			}
			return err
		}
		idx, err := ioutil.ReadAll(r)
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(outFile, idx, 0o644); err != nil {
			return err
		}
		r.Close()
		log.Infof("Wrote %v", outFile)
	}

	// Run our action
	if err := f(); err != nil {
		return fmt.Errorf("action failed: %v", err)
	}

	// Now we want to (try to) write it
	o := obj
	if generation > 0 {
		o = o.If(storage.Conditions{GenerationMatch: generation})
	}
	w := o.NewWriter(context.Background())

	// Ensure we do not cache. This would be fine for normal users reading, but it ends up making the release process
	// break if we have multiple releases too quickly (default cache is 1hr).
	w.CacheControl = "no-cache, max-age=0, no-transform"

	// https://helm.sh/docs/topics/chart_repository/#ordinary-web-servers
	w.ContentType = "text/yaml"

	res, err := os.Open(outFile)
	if err != nil {
		return fmt.Errorf("failed to open %v: %v", res.Name(), err)
	}
	if _, err := io.Copy(w, bufio.NewReader(res)); err != nil {
		return fmt.Errorf("failed writing %v: %v", res.Name(), err)
	}
	if err := w.Close(); err != nil {
		gerr, ok := err.(*googleapi.Error)
		if ok && gerr.Code == 412 {
			return ErrIndexOutOfDate
		}
		return fmt.Errorf("failed to close bucket: %v", err)
	}
	if attr, err := obj.Attrs(context.Background()); err != nil {
		log.Errorf("failed to get attributes: %v", err)
	} else {
		log.Infof("Object %v now has generation %d", objName, attr.Generation)
	}
	return nil
}

var ErrIndexOutOfDate = errors.New("index is out-of-date")
