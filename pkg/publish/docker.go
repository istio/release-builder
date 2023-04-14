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
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"istio.io/pkg/log"
	"istio.io/release-builder/pkg/model"
	"istio.io/release-builder/pkg/util"
)

// Image defines a single docker image. There are potentially many Image outputs for each .tar.gz - this
// represents the fully expanded form.
// Example:
//
//	Image{
//		 OriginalTag: "localhost/proxyv2:original", // Hub from manifest
//		 NewTag:      "gcr.io/istio-release/proxyv2:new", // Hub from --dockerhubs and --dockertags
//		 Variant      "",
//		 Image        "proxyv2",
//	}
//
// An image also logically containers an "Architecture" component. However, we want to merge based on arch, so we
// use this as a `map[Image][]architecture{}
type Image struct {
	OriginalTag string
	NewTag      string
	Variant     string // May be empty for default variant
	Image       string
}

func (i Image) OriginalReference(arch string) string {
	return fmt.Sprintf("%s%s%s", i.OriginalTag, toSuffix(i.Variant), toSuffix(arch))
}

func (i Image) NewReference(arch string) string {
	return fmt.Sprintf("%s%s%s", i.NewTag, toSuffix(i.Variant), toSuffix(arch))
}

func (i Image) VariantSuffix() string {
	if i.Variant != "" {
		return "-" + i.Variant
	}
	return ""
}

func toSuffix(s string) string {
	if s == "" {
		return ""
	}
	return "-" + s
}

// Docker publishes all images to the given hub
func Docker(manifest model.Manifest, hub string, tags []string, cosignkey string) error {
	if len(tags) == 0 {
		tags = []string{manifest.Version}
	}
	dockerArchives, err := os.ReadDir(path.Join(manifest.Directory, "docker"))
	if err != nil {
		return fmt.Errorf("failed to read docker output of release: %v", err)
	}

	// Only attempt to sign images if a valid cosign key is provided and we are
	// able to run 'cosign public-key <key>'.
	cosignEnabled := false
	if cosignkey != "" {
		if err := util.VerboseCommand("cosign", "public-key", "--key", cosignkey, "-y").Run(); err != nil {
			log.Errorf("Argument '--cosignkey' nonempty but unable to access key %v, disabling signing.", err)
		} else {
			cosignEnabled = true
		}
	}

	// As inputs, we have a variety of tar.gz files emitted from `docker save`.
	// Our goal is to take these, and potentially mangle the hub/tags, and push to the real registry.
	// This becomes more complex because for multi-arch images, we want to push a single manifest but we have multiple tar files (one per arch).

	// first, we will load all our images into the local docker daemon, and setup an index of Image -> architectures.
	// Each entry will result in one upstream tag created.
	images := map[Image][]string{}
	for _, f := range dockerArchives {
		if !strings.HasSuffix(f.Name(), "tar.gz") {
			return fmt.Errorf("invalid image found in docker folder: %v", f.Name())
		}
		if err := util.VerboseCommand("docker", "load", "-i", path.Join(manifest.Directory, "docker", f.Name())).Run(); err != nil {
			return fmt.Errorf("failed to load docker image %v: %v", f.Name(), err)
		}
		imageName, variant, arch := getImageNameVariant(f.Name())
		variants := []string{variant}
		for _, tag := range tags {
			for _, variant := range variants {
				img := Image{
					OriginalTag: fmt.Sprintf("%s/%s:%s", manifest.Docker, imageName, manifest.Version),
					NewTag:      fmt.Sprintf("%s/%s:%s", hub, imageName, tag),
					Variant:     variant,
					Image:       imageName,
				}
				images[img] = append(images[img], arch)
			}
		}
	}

	// Now that we have the desired outputs, start pushing
	for img, archs := range images {
		// Split case for simple images (single arch) vs multi-arch manifests.
		if len(archs) == 1 {
			arch := archs[0]
			// Single architecture. We just want to push directly
			// Single arch, push directly
			if err := util.VerboseCommand("docker", "tag", img.OriginalReference(arch), img.NewReference(arch)).Run(); err != nil {
				return fmt.Errorf("failed to tag docker image %v->%v: %v", img.OriginalReference(arch), img.NewReference(arch), err)
			}

			if err := util.VerboseCommand("docker", "push", img.NewReference(arch)).Run(); err != nil {
				return fmt.Errorf("failed to push docker image %v: %v", img.NewReference(arch), err)
			}

			// Sign images *after* push -- cosign only works against real
			// repositories (not valid against tarballs)
			if cosignEnabled {
				imgRef, err := name.ParseReference(img.NewReference(arch))
				if err != nil {
					return fmt.Errorf("failed to parse image reference %v: %v", img.NewReference(arch), err)
				}
				newImg, err := remote.Image(imgRef, remote.WithAuthFromKeychain(authn.DefaultKeychain))
				if err != nil {
					return fmt.Errorf("failed to load %v: %v", imgRef, err)
				}
				digest, err := newImg.Digest()
				if err != nil {
					return fmt.Errorf("failed to get digest for %v: %v", imgRef, err)
				}
				// We need to return the digest of the manifest, not the image. This is because the manifest is what is signed.
				// This should return something like `gcr.io/istio-testing/pilot@sha256:1234`
				if err := util.VerboseCommand("cosign", "sign", "--key", cosignkey, imgRef.Context().String()+"@"+digest.String(), "-y", "--recursive").Run(); err != nil {
					return fmt.Errorf("failed to sign image %v with key %v: %v", img.NewReference(arch), cosignkey, err)
				}
			}
		} else {
			localImages := []string{}
			for _, arch := range archs {
				localImages = append(localImages, img.OriginalReference(arch))
			}
			digest, err := publishManifest(img.NewReference(""), localImages)
			if err != nil {
				return err
			}
			if cosignEnabled {
				if err := util.VerboseCommand("cosign", "sign", "--key", cosignkey, digest, "-y", "--recursive").Run(); err != nil {
					return fmt.Errorf("failed to sign image %v with key %v: %v", digest, cosignkey, err)
				}
			}
		}
	}
	return nil
}

// publishManifest packages each image in `images` into a single manifest, and pushes to `manifest`.
func publishManifest(manifest string, images []string) (string, error) {
	log.Infof("creating manifest %v from %v", manifest, images)
	// Typically we could just use `docker manifest create manifest images...`. However, we need to actually
	// push source images first. We want to push these without a tag, so users never use them. Docker cannot
	// push directly by tag, so here we are...
	craneImages := []v1.Image{}
	for _, image := range images {
		tagRef, err := name.ParseReference(image)
		if err != nil {
			return "", fmt.Errorf("failed to parse %v: %v", image, err)
		}
		log.Infof("starting push of %v for manifest (without tag)", tagRef)
		img, err := daemon.Image(tagRef)
		if err != nil {
			return "", fmt.Errorf("failed to load %v: %v", image, err)
		}
		digest, err := img.Digest()
		if err != nil {
			return "", fmt.Errorf("failed to get digest for %v: %v", image, err)
		}
		digestRef, err := name.NewDigest(fmt.Sprintf("%s@%s", tagRef.Context(), digest.String()))
		if err != nil {
			return "", fmt.Errorf("failed to build digest reference for %v: %v", image, err)
		}
		if err := remote.Write(digestRef, img, remote.WithAuthFromKeychain(authn.DefaultKeychain)); err != nil {
			return "", fmt.Errorf("failed to push %v: %v", image, err)
		}
		craneImages = append(craneImages, img)
		log.Infof("pushed %v for manifest", digestRef)
	}
	// Now all the images are in the registry, build the manifest. We can't just utilize `docker manifest create`,
	// since that would be too easy - docker requires the images are in the local daemon, and loading them changes the digest.
	// Instead, we do it ourselves again.
	var index v1.ImageIndex = empty.Index
	index = mutate.IndexMediaType(index, types.DockerManifestList)
	for _, img := range craneImages {
		mt, err := img.MediaType()
		if err != nil {
			return "", fmt.Errorf("failed to get mediatype: %w", err)
		}

		h, err := img.Digest()
		if err != nil {
			return "", fmt.Errorf("failed to compute digest: %w", err)
		}

		size, err := img.Size()
		if err != nil {
			return "", fmt.Errorf("failed to compute size: %w", err)
		}
		cfg, err := img.ConfigFile()
		if err != nil {
			return "", fmt.Errorf("failed to get config file: %w", err)
		}
		index = mutate.AppendManifests(index, mutate.IndexAddendum{
			Add: img,
			Descriptor: v1.Descriptor{
				MediaType: mt,
				Size:      size,
				Digest:    h,
				Platform: &v1.Platform{
					Architecture: cfg.Architecture,
					OS:           cfg.OS,
					OSVersion:    cfg.OSVersion,
					Variant:      cfg.Variant,
					Features:     nil,
				},
			},
		})
	}
	manifestRef, err := name.ParseReference(manifest)
	if err != nil {
		return "", fmt.Errorf("failed to parse %v: %v", manifestRef, err)
	}
	if err := remote.MultiWrite(map[name.Reference]remote.Taggable{manifestRef: index}, remote.WithAuthFromKeychain(authn.DefaultKeychain)); err != nil {
		return "", fmt.Errorf("failed to push %v: %v", manifestRef, err)
	}
	digest, err := index.Digest()
	if err != nil {
		return "", fmt.Errorf("failed to get digest for %v: %v", manifestRef, err)
	}
	// We need to return the digest of the manifest, not the image. This is because the manifest is what is signed.
	// This should return something like `gcr.io/istio-testing/pilot@sha256:1234`
	return manifestRef.Context().String() + "@" + digest.String(), nil
}

// getImageNameVariant determines the name of the image (eg, pilot) and variant (eg, distroless).
// This is derived from the file name.
func getImageNameVariant(fname string) (name string, variant string, arch string) {
	imageName := strings.Split(fname, ".")[0]
	if match, _ := filepath.Match("*-arm64", imageName); match {
		arch = "arm64"
		imageName = strings.TrimSuffix(imageName, "-arm64")
	}
	if match, _ := filepath.Match("*-distroless", imageName); match {
		variant = "distroless"
		imageName = strings.TrimSuffix(imageName, "-distroless")
	}
	if match, _ := filepath.Match("*-debug", imageName); match {
		variant = "debug"
		imageName = strings.TrimSuffix(imageName, "-debug")
	}
	name = imageName
	return
}
