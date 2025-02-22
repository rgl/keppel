/******************************************************************************
*
*  Copyright 2020 SAP SE
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

package client

import (
	"io"
	"net/http"
	"strconv"

	"github.com/docker/distribution"
	"github.com/opencontainers/go-digest"

	"github.com/sapcc/keppel/internal/keppel"
)

// DownloadBlob fetches a blob's contents from this repository. If an error is
// returned, it's usually a *keppel.RegistryV2Error.
func (c *RepoClient) DownloadBlob(blobDigest digest.Digest) (contents io.ReadCloser, sizeBytes uint64, returnErr error) {
	resp, err := c.doRequest(repoRequest{
		Method:       "GET",
		Path:         "blobs/" + blobDigest.String(),
		ExpectStatus: http.StatusOK,
	})
	if err != nil {
		return nil, 0, err
	}
	sizeBytes, err = strconv.ParseUint(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		resp.Body.Close()
		return nil, 0, err
	}
	return resp.Body, sizeBytes, nil
}

// DownloadManifestOpts appears in func DownloadManifest.
type DownloadManifestOpts struct {
	DoNotCountTowardsLastPulled bool
	ExtraHeaders                http.Header
}

// DownloadManifest fetches a manifest from this repository. If an error is
// returned, it's usually a *keppel.RegistryV2Error.
func (c *RepoClient) DownloadManifest(reference keppel.ManifestReference, opts *DownloadManifestOpts) (contents []byte, mediaType string, returnErr error) {
	if opts == nil {
		opts = &DownloadManifestOpts{}
	}

	hdr := http.Header{"Accept": distribution.ManifestMediaTypes()}
	if opts.DoNotCountTowardsLastPulled {
		hdr.Set("X-Keppel-No-Count-Towards-Last-Pulled", "1")
	}
	for k, v := range opts.ExtraHeaders {
		if len(v) > 0 {
			hdr[k] = v
		}
	}

	resp, err := c.doRequest(repoRequest{
		Method:       "GET",
		Path:         "manifests/" + reference.String(),
		Headers:      hdr,
		ExpectStatus: http.StatusOK,
	})
	if err != nil {
		return nil, "", err
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err == nil {
		err = resp.Body.Close()
	} else {
		resp.Body.Close()
	}
	if err != nil {
		return nil, "", err
	}

	return respBytes, resp.Header.Get("Content-Type"), nil
}
