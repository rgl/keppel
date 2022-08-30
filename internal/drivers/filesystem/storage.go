/******************************************************************************
*
*  Copyright 2022 ruilopes.com
*
*  Licensed under the Apache License, Version 2.0 (the "License");
*  you may not use this file except in compliance with the License.
*  You may obtain a copy of the License at
*
*      http://www.apache.org/licenses/LICENSE-2.0
*
*  Unless required by applicable law or agreed to in writing, software
*  distributed under the License is distributed on an "AS IS" BASIS,
*  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*  See the License for the specific language governing permissions and
*  limitations under the License.
*
******************************************************************************/

// TODO the error returned here are displayed to the user as-is. is that a problem? like, leaking path names?
// TODO wrap all the errors?

package filesystem

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sapcc/keppel/internal/keppel"
	"github.com/spf13/afero"
)

func init() {
	keppel.RegisterStorageDriver("filesystem", func(_ keppel.AuthDriver, _ keppel.Configuration) (keppel.StorageDriver, error) {
		rootPath := os.Getenv("KEPPEL_FILESYSTEM_PATH")
		if rootPath == "" {
			return nil, fmt.Errorf("the KEPPEL_FILESYSTEM_PATH environment variable must not be empty")
		}
		return &StorageDriver{
			root: afero.NewBasePathFs(afero.NewOsFs(), rootPath),
		}, nil
	})
}

// StorageDriver (driver ID "filesystem") is a keppel.StorageDriver that stores its contents in the local filesystem.
type StorageDriver struct {
	root afero.Fs
}

func blobBasePath(account keppel.Account) string {
	return fmt.Sprintf("%s/%s/b", account.AuthTenantID, account.Name)
}

func blobPath(account keppel.Account, storageID string) string {
	return fmt.Sprintf("%s/%s/b/%s", account.AuthTenantID, account.Name, storageID)
}

func manifestBasePath(account keppel.Account) string {
	return fmt.Sprintf("%s/%s/m", account.AuthTenantID, account.Name)
}

func manifestPath(account keppel.Account, repoName, digest string) string {
	return fmt.Sprintf("%s/%s/m/%s/%s", account.AuthTenantID, account.Name, repoName, digest)
}

// AppendToBlob implements the keppel.StorageDriver interface.
func (d *StorageDriver) AppendToBlob(account keppel.Account, storageID string, chunkNumber uint32, chunkLength *uint64, chunk io.Reader) error {
	path := blobPath(account, storageID)
	tmpPath := fmt.Sprintf("%s.tmp", path)
	flags := os.O_APPEND
	if chunkNumber == 1 {
		err := d.root.MkdirAll(filepath.Dir(tmpPath), 0700)
		if err != nil {
			return err
		}
		flags = flags | os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	f, err := d.root.OpenFile(tmpPath, flags, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, chunk)
	if err != nil {
		return err
	}
	return nil
}

// FinalizeBlob implements the keppel.StorageDriver interface.
func (d *StorageDriver) FinalizeBlob(account keppel.Account, storageID string, chunkCount uint32) error {
	path := blobPath(account, storageID)
	tmpPath := fmt.Sprintf("%s.tmp", path)
	return d.root.Rename(tmpPath, path)
}

// AbortBlobUpload implements the keppel.StorageDriver interface.
func (d *StorageDriver) AbortBlobUpload(account keppel.Account, storageID string, chunkCount uint32) error {
	path := blobPath(account, storageID)
	tmpPath := fmt.Sprintf("%s.tmp", path)
	return d.root.Remove(tmpPath)
}

// ReadBlob implements the keppel.StorageDriver interface.
func (d *StorageDriver) ReadBlob(account keppel.Account, storageID string) (io.ReadCloser, uint64, error) {
	path := blobPath(account, storageID)
	f, err := d.root.Open(path)
	if err != nil {
		return nil, 0, err
	}
	stat, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, err
	}
	return f, uint64(stat.Size()), nil
}

// URLForBlob implements the keppel.StorageDriver interface.
func (d *StorageDriver) URLForBlob(account keppel.Account, storageID string) (string, error) {
	return "", keppel.ErrCannotGenerateURL
}

// DeleteBlob implements the keppel.StorageDriver interface.
func (d *StorageDriver) DeleteBlob(account keppel.Account, storageID string) error {
	path := blobPath(account, storageID)
	return d.root.Remove(path)
}

// ReadManifest implements the keppel.StorageDriver interface.
func (d *StorageDriver) ReadManifest(account keppel.Account, repoName, digest string) ([]byte, error) {
	path := manifestPath(account, repoName, digest)
	return afero.ReadFile(d.root, path)
}

// WriteManifest implements the keppel.StorageDriver interface.
func (d *StorageDriver) WriteManifest(account keppel.Account, repoName, digest string, contents []byte) error {
	path := manifestPath(account, repoName, digest)
	tmpPath := fmt.Sprintf("%s.tmp", path)
	err := d.root.MkdirAll(filepath.Dir(tmpPath), 0700)
	if err != nil {
		return err
	}
	err = afero.WriteFile(d.root, tmpPath, contents, 0600)
	if err != nil {
		return err
	}
	return d.root.Rename(tmpPath, path)
}

// DeleteManifest implements the keppel.StorageDriver interface.
func (d *StorageDriver) DeleteManifest(account keppel.Account, repoName, digest string) error {
	path := manifestPath(account, repoName, digest)
	return d.root.Remove(path)
}

// ListStorageContents implements the keppel.StorageDriver interface.
func (d *StorageDriver) ListStorageContents(account keppel.Account) ([]keppel.StoredBlobInfo, []keppel.StoredManifestInfo, error) {
	blobs, err := d.getBlobs(account)
	if err != nil {
		return nil, nil, err
	}
	manifests, err := d.getManifests(account)
	if err != nil {
		return nil, nil, err
	}
	return blobs, manifests, nil
}

func (d *StorageDriver) getBlobs(account keppel.Account) ([]keppel.StoredBlobInfo, error) {
	var blobs []keppel.StoredBlobInfo
	directory, err := d.root.Open(blobBasePath(account))
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	names, err := directory.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		if strings.HasSuffix(name, ".tmp") {
			continue
		}
		blobs = append(blobs, keppel.StoredBlobInfo{
			StorageID: name,
		})
	}
	return blobs, nil
}

func (d *StorageDriver) getManifests(account keppel.Account) ([]keppel.StoredManifestInfo, error) {
	var manifests []keppel.StoredManifestInfo
	directory, err := d.root.Open(manifestBasePath(account))
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	repos, err := directory.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	for _, repo := range repos {
		repoManifests, err := d.getRepoManifests(account, repo)
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, repoManifests...)
	}
	return manifests, nil
}

func (d *StorageDriver) getRepoManifests(account keppel.Account, repo string) ([]keppel.StoredManifestInfo, error) {
	var manifests []keppel.StoredManifestInfo
	directory, err := d.root.Open(filepath.Join(manifestBasePath(account), repo))
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	digests, err := directory.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	for _, digest := range digests {
		if strings.HasSuffix(digest, ".tmp") {
			continue
		}
		manifests = append(manifests, keppel.StoredManifestInfo{
			RepoName: repo,
			Digest:   digest,
		})
	}
	return manifests, nil
}

// CleanupAccount implements the keppel.StorageDriver interface.
func (d *StorageDriver) CleanupAccount(account keppel.Account) error {
	//double-check that cleanup order is right; when the account gets deleted,
	//all blobs and manifests must have been deleted from it before
	storedBlobs, storedManifests, err := d.ListStorageContents(account)
	if len(storedBlobs) > 0 {
		return fmt.Errorf(
			"found undeleted blob during CleanupAccount: storageID = %q",
			storedBlobs[0].StorageID,
		)
	}
	if len(storedManifests) > 0 {
		return fmt.Errorf(
			"found undeleted manifest during CleanupAccount: %s@%s",
			storedManifests[0].RepoName,
			storedManifests[0].Digest,
		)
	}
	return err
}
