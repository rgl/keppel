/******************************************************************************
*
*  Copyright 2018-2019 SAP SE
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

package keppelv1

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/sapcc/go-bits/respondwith"

	"github.com/sapcc/keppel/internal/auth"
	"github.com/sapcc/keppel/internal/keppel"
	"github.com/sapcc/keppel/internal/processor"
)

// API contains state variables used by the Keppel V1 API implementation.
type API struct {
	cfg        keppel.Configuration
	authDriver keppel.AuthDriver
	fd         keppel.FederationDriver
	sd         keppel.StorageDriver
	icd        keppel.InboundCacheDriver
	db         *keppel.DB
	auditor    keppel.Auditor
}

// NewAPI constructs a new API instance.
func NewAPI(cfg keppel.Configuration, ad keppel.AuthDriver, fd keppel.FederationDriver, sd keppel.StorageDriver, icd keppel.InboundCacheDriver, db *keppel.DB, auditor keppel.Auditor) *API {
	return &API{cfg, ad, fd, sd, icd, db, auditor}
}

// AddTo implements the api.API interface.
func (a *API) AddTo(r *mux.Router) {
	r.Methods("GET").Path("/keppel/v1").HandlerFunc(a.handleGetAPIInfo)

	//NOTE: Keppel account names are severely restricted because we used to
	//derive Postgres database names from them.
	r.Methods("GET").Path("/keppel/v1/accounts").HandlerFunc(a.handleGetAccounts)
	r.Methods("GET").Path("/keppel/v1/accounts/{account:[a-z0-9-]{1,48}}").HandlerFunc(a.handleGetAccount)
	r.Methods("PUT").Path("/keppel/v1/accounts/{account:[a-z0-9-]{1,48}}").HandlerFunc(a.handlePutAccount)
	r.Methods("DELETE").Path("/keppel/v1/accounts/{account:[a-z0-9-]{1,48}}").HandlerFunc(a.handleDeleteAccount)
	r.Methods("POST").Path("/keppel/v1/accounts/{account:[a-z0-9-]{1,48}}/sublease").HandlerFunc(a.handlePostAccountSublease)

	r.Methods("GET").Path("/keppel/v1/accounts/{account:[a-z0-9-]{1,48}}/repositories/{repo_name:.+}/_manifests").HandlerFunc(a.handleGetManifests)
	r.Methods("DELETE").Path("/keppel/v1/accounts/{account:[a-z0-9-]{1,48}}/repositories/{repo_name:.+}/_manifests/{digest}").HandlerFunc(a.handleDeleteManifest)
	r.Methods("GET").Path("/keppel/v1/accounts/{account:[a-z0-9-]{1,48}}/repositories/{repo_name:.+}/_manifests/{digest}/vulnerability_report").HandlerFunc(a.handleGetVulnerabilityReport)
	r.Methods("DELETE").Path("/keppel/v1/accounts/{account:[a-z0-9-]{1,48}}/repositories/{repo_name:.+}/_tags/{tag_name}").HandlerFunc(a.handleDeleteTag)

	r.Methods("GET").Path("/keppel/v1/accounts/{account:[a-z0-9-]{1,48}}/repositories").HandlerFunc(a.handleGetRepositories)
	r.Methods("DELETE").Path("/keppel/v1/accounts/{account:[a-z0-9-]{1,48}}/repositories/{repo_name:.+}").HandlerFunc(a.handleDeleteRepository)

	r.Methods("GET").Path("/keppel/v1/peers").HandlerFunc(a.handleGetPeers)

	r.Methods("GET").Path("/keppel/v1/quotas/{auth_tenant_id}").HandlerFunc(a.handleGetQuotas)
	r.Methods("PUT").Path("/keppel/v1/quotas/{auth_tenant_id}").HandlerFunc(a.handlePutQuotas)
}

func (a *API) processor() *processor.Processor {
	return processor.New(a.cfg, a.db, a.sd, a.icd, a.auditor)
}

func (a *API) handleGetAPIInfo(w http.ResponseWriter, r *http.Request) {
	respondwith.JSON(w, http.StatusOK, struct {
		AuthDriverName string `json:"auth_driver"`
	}{
		AuthDriverName: a.authDriver.DriverName(),
	})
}

func respondWithAuthError(w http.ResponseWriter, err *keppel.RegistryV2Error) bool {
	if err == nil {
		return false
	}
	err.WriteAsTextTo(w)
	w.Write([]byte("\n"))
	return true
}

func authTenantScope(perm keppel.Permission, authTenantID string) auth.ScopeSet {
	return auth.NewScopeSet(auth.Scope{
		ResourceType: "keppel_auth_tenant",
		ResourceName: authTenantID,
		Actions:      []string{string(perm)},
	})
}

func accountScopeFromRequest(r *http.Request, perm keppel.Permission) auth.ScopeSet {
	return auth.NewScopeSet(auth.Scope{
		ResourceType: "keppel_account",
		ResourceName: mux.Vars(r)["account"],
		Actions:      []string{string(perm)},
	})
}

func accountScopes(perm keppel.Permission, accounts ...keppel.Account) auth.ScopeSet {
	scopes := make([]auth.Scope, len(accounts))
	for idx, account := range accounts {
		scopes[idx] = auth.Scope{
			ResourceType: "keppel_account",
			ResourceName: account.Name,
			Actions:      []string{string(perm)},
		}
	}
	return auth.NewScopeSet(scopes...)
}

func repoScopeFromRequest(r *http.Request, perm keppel.Permission) auth.ScopeSet {
	vars := mux.Vars(r)
	return auth.NewScopeSet(auth.Scope{
		ResourceType: "repository",
		ResourceName: fmt.Sprintf("%s/%s", vars["account"], vars["repo_name"]),
		Actions:      []string{string(perm)},
	})
}

func (a *API) authenticateRequest(w http.ResponseWriter, r *http.Request, ss auth.ScopeSet) *auth.Authorization {
	authz, rerr := auth.IncomingRequest{
		HTTPRequest:          r,
		Scopes:               ss,
		CorrectlyReturn403:   true,
		PartialAccessAllowed: r.URL.Path == "/keppel/v1/accounts",
	}.Authorize(a.cfg, a.authDriver, a.db)
	if rerr != nil {
		rerr.WriteAsTextTo(w)
		return nil
	}
	return authz
}

func (a *API) findAccountFromRequest(w http.ResponseWriter, r *http.Request) *keppel.Account {
	accountName := mux.Vars(r)["account"]
	account, err := keppel.FindAccount(a.db, accountName)
	if respondwith.ErrorText(w, err) {
		return nil
	}
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return nil
	}
	return account
}

func (a *API) findRepositoryFromRequest(w http.ResponseWriter, r *http.Request, account keppel.Account) *keppel.Repository {
	repoName := mux.Vars(r)["repo_name"]
	if !isValidRepoName(repoName) {
		http.Error(w, "not found", http.StatusNotFound)
		return nil
	}

	repo, err := keppel.FindRepository(a.db, repoName, account)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return nil
	}
	if respondwith.ErrorText(w, err) {
		return nil
	}
	return repo
}

func isValidRepoName(name string) bool {
	if name == "" {
		return false
	}
	for _, pathComponent := range strings.Split(name, `/`) {
		if !keppel.RepoPathComponentRx.MatchString(pathComponent) {
			return false
		}
	}
	return true
}

type paginatedQuery struct {
	SQL         string
	MarkerField string
	Options     url.Values
	BindValues  []interface{}
}

func (q paginatedQuery) Prepare() (modifiedSQLQuery string, modifiedBindValues []interface{}, limit uint64, err error) {
	//hidden feature: allow lowering the default limit with ?limit= (we only
	//really use this for the unit tests)
	limit = uint64(1000)
	if limitStr := q.Options.Get("limit"); limitStr != "" {
		limitVal, err := strconv.ParseUint(limitStr, 10, 64)
		if err != nil {
			return "", nil, 0, err
		}
		if limitVal < limit { //never allow more than 1000 results at once
			limit = limitVal
		}
	}
	//fetch one more than `limit`: otherwise we cannot distinguish between a
	//truncated 1000-row result and a non-truncated 1000-row result
	query := strings.Replace(q.SQL, `$LIMIT`, strconv.FormatUint(limit+1, 10), 1)

	marker := q.Options.Get("marker")
	if marker == "" {
		query = strings.Replace(query, `$CONDITION`, `TRUE`, 1)
		return query, q.BindValues, limit, nil
	}
	query = strings.Replace(query, `$CONDITION`, q.MarkerField+` > $2`, 1)
	return query, append(q.BindValues, marker), limit, nil
}
