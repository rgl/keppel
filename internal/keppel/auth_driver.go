/*******************************************************************************
*
* Copyright 2018 SAP SE
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You should have received a copy of the License along with this
* program. If not, you may obtain a copy of the License at
*
*     http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*
*******************************************************************************/

package keppel

import (
	"encoding/base64"
	"errors"
	"net/http"

	"github.com/go-redis/redis/v8"
)

// Permission is an enum used by AuthDriver.
type Permission string

const (
	//CanViewAccount is the permission for viewing account metadata.
	CanViewAccount Permission = "view"
	//CanPullFromAccount is the permission for pulling images from this account.
	CanPullFromAccount Permission = "pull"
	//CanPushToAccount is the permission for pushing images to this account.
	CanPushToAccount Permission = "push"
	//CanDeleteFromAccount is the permission for deleting manifests from this account.
	CanDeleteFromAccount Permission = "delete"
	//CanChangeAccount is the permission for creating and updating accounts.
	CanChangeAccount Permission = "change"
	//CanViewQuotas is the permission for viewing an auth tenant's quotas.
	CanViewQuotas Permission = "viewquota"
	//CanChangeQuotas is the permission for changing an auth tenant's quotas.
	CanChangeQuotas Permission = "changequota"
	//CanAdministrateKeppel is a global permission (not tied to any auth tenant) for certain administrative tasks in Keppel.
	CanAdministrateKeppel Permission = "keppeladmin"
)

// AuthDriver represents an authentication backend that supports multiple
// tenants. A tenant is a scope where users can be authorized to perform certain
// actions. For example, in OpenStack, a Keppel tenant is a Keystone project.
type AuthDriver interface {
	//DriverName returns the name of the auth driver as specified in
	//RegisterAuthDriver() and, therefore, the KEPPEL_AUTH_DRIVER variable.
	DriverName() string

	//ValidateTenantID checks if the given string is a valid tenant ID. If so,
	//nil shall be returned. If not, the returned error shall explain why the ID
	//is not valid. The driver implementor can decide how thorough this check
	//shall be: It can be anything from "is not empty" to "matches regex" to
	//"exists in the auth database".
	ValidateTenantID(tenantID string) error

	//AuthenticateUser authenticates the user identified by the given username
	//and password. Note that usernames may not contain colons, because
	//credentials are encoded by clients in the "username:password" format.
	AuthenticateUser(userName, password string) (UserIdentity, *RegistryV2Error)
	//AuthenticateUserFromRequest reads credentials from the given incoming HTTP
	//request to authenticate the user which makes this request. The
	//implementation shall follow the conventions of the concrete backend, e.g. a
	//OAuth backend could try to read a Bearer token from the Authorization
	//header, whereas an OpenStack auth driver would look for a Keystone token in the
	//X-Auth-Token header.
	//
	//If the request contains no auth headers at all, (nil, nil) shall be
	//returned to trigger the codepath for anonymous users.
	AuthenticateUserFromRequest(r *http.Request) (UserIdentity, *RegistryV2Error)
}

var authDriverFactories = make(map[string]func(*redis.Client) (AuthDriver, error))

// NewAuthDriver creates a new AuthDriver using one of the factory functions
// registered with RegisterAuthDriver().
func NewAuthDriver(name string, rc *redis.Client) (AuthDriver, error) {
	factory := authDriverFactories[name]
	if factory != nil {
		return factory(rc)
	}
	return nil, errors.New("no such auth driver: " + name)
}

// RegisterAuthDriver registers an AuthDriver. Call this from func init() of the
// package defining the AuthDriver.
//
// Warning: The *redis.Client argument of the factory function is optional! Only
// use it for caching authorizations if it is non-nil.
func RegisterAuthDriver(name string, factory func(*redis.Client) (AuthDriver, error)) {
	if _, exists := authDriverFactories[name]; exists {
		panic("attempted to register multiple auth drivers with name = " + name)
	}
	authDriverFactories[name] = factory
}

// BuildBasicAuthHeader constructs the value of an "Authorization" HTTP header for the given basic auth credentials.
func BuildBasicAuthHeader(userName, password string) string {
	creds := userName + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
}
