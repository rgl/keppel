# Keppel, a multi-tenant container image registry

[![CI](https://github.com/sapcc/keppel/actions/workflows/ci.yaml/badge.svg)](https://github.com/sapcc/keppel/actions/workflows/ci.yaml)
[![Conformance Test](https://github.com/sapcc/keppel/actions/workflows/oci-distribution-conformance.yml/badge.svg)](https://github.com/sapcc/keppel/actions/workflows/oci-distribution-conformance.yml)
[![Coverage Status](https://coveralls.io/repos/github/sapcc/keppel/badge.svg?branch=master)](https://coveralls.io/github/sapcc/keppel?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/sapcc/keppel)](https://goreportcard.com/report/github.com/sapcc/keppel)

In this document:

- [Overview](#overview)
- [History](#history)
- [Terminology](#terminology)
- [Usage](#usage)
- [Usage (docker compose)](#usage-docker-compose)

In other documents:

- [Operator guide](./docs/operator-guide.md)
- [API specification](./docs/api-spec.md)
- [Notes for developers/contributors](./CONTRIBUTING.md)

## Overview

When working with the container ecosystem (Docker, Kubernetes, etc.), you need a place to store your container images.
The default choice is the [Docker Registry](https://github.com/docker/distribution), but a Docker Registry alone is not
enough in productive deployments because you also need a compatible OAuth2 provider. A popular choice is
[Harbor](https://goharbor.io), which bundles a Docker Registry, an auth provider, and some other tools. Another choice
is [Quay](https://github.com/quay/quay), which implements the registry protocol itself, but is otherwise very similar to
Harbor.

However, Harbor's architecture is limited by its use of a single registry that is shared between all users. Most
importantly, by putting the images of all users in the same storage, quota and usage tracking gets unnecessarily
complicated. Keppel instead uses multi-tenant-aware storage drivers so that each customer gets their own separate
storage space. Storage quota and usage can therefore be tracked by the backing storage. This orchestration is completely
transparent to the user: A unified API endpoint is provided that multiplexes requests to their respective registry
instances.

Keppel fully implements the [OCI Distribution API][dist-api], the standard API for container image registries. It also
provides a [custom API](docs/api-spec.md) to control the multitenancy added by Keppel and to expose additional metadata
about repositories, manifests and tags. Other unique features of Keppel include:

- **cross-regional federation**: Keppel instances running in different geographical regions or different network
  segments can share their account name space and provide seamless replication between accounts of the same name on
  different instances.
- **online garbage collection**: Unlike Docker Registry, Keppel can perform all garbage collection tasks without
  scheduled downtime or any other form of operator intervention.
- **vulnerability scanning**: Keppel can use [Clair](https://quay.github.io/clair/) to perform vulnerability scans on
  its contents.

[dist-api]: https://github.com/opencontainers/distribution-spec

## History

In its first year, leading up to 1.0, Keppel used to orchestrate a fleet of docker-registry instances to provide the
OCI Distribution API. We hoped to save ourselves the work of reimplementing the full Distribution API, since Keppel
would only have to reverse-proxy customer requests into their respective docker-registry. Over time, as Keppel's feature
set grew, more and more API requests were intercepted to track metadata, validate requests and so forth. We ended up
scrapping the docker-registry fleet entirely to make Keppel much easier to deploy and manage. It's now conceptually more
similar to Quay than to Harbor, but retains its unique multi-tenant storage architecture.

## Terminology

Within Keppel, an **account** is a namespace that gets its own backing storage. The account name is the first path
element in the name of a repository. For example, consider the image `keppel.example.com/foo/bar:latest`. It's
repository name is `foo/bar`, which means it's located in the Keppel account `foo`.

Access is controlled by the account's **auth tenant** or just **tenant**. Note that tenants and accounts are separate
concepts: An account corresponds to one backing storage, and a tenant corresponds to an authentication/authorization
scope. Each account is linked to exactly one auth tenant, but there can be multiple accounts linked to the same auth
tenant.

Inside an account, you will find **repositories** containing **blobs**, **manifests** and **tags**. The meaning of those
terms is the same as for any other Docker registry or OCI image registry, and is defined in the [OCI Distribution API
specification][dist-api].

## Usage

Build with `make`, install with `make install` or `docker build`. The resulting `keppel` binary contains client commands
as well as server commands.

- For how to use the client commands, run `keppel --help`.
- For how to deploy the server components, please refer to the [operator guide](./docs/operator-guide.md).

## Usage (docker compose)

Configure your `hosts` file:

```bash
echo '127.0.0.1 keppel.test' | sudo bash -c 'cat >>/etc/hosts'
```

Start the environment:

```bash
docker compose up --build
```

Execute the next commands in a new shell.

Create the `keppel` tenant and a couple of accounts:

```bash
docker compose exec --user root keppel chown nobody:nobody /data
docker compose exec --user postgres postgres psql keppel <<'EOF'
insert into quotas(auth_tenant_id, manifests) values('keppel', 10000);
insert into accounts(auth_tenant_id, name) values('keppel', 'ruilopes');
insert into accounts(auth_tenant_id, name) values('keppel', 'library');
EOF
```

Push an image:

```bash
source_image='docker.io/ruilopes/example-docker-buildx-go:v1.10.0'
image='keppel.test:9006/ruilopes/example-docker-buildx-go:v1.10.0'
platform='linux/amd64'
#platform='windows/amd64:10.0.20348.825'
#platform='all'
crane manifest --insecure "$source_image" | jq
crane copy --insecure --platform "$platform" "$source_image" "$image"
crane manifest --insecure "$image" | jq
```

List what ended up in the keppel `data` volume:

```bash
docker compose exec keppel find /data -type f
```

It should be something alike:

```
/data/keppel/ruilopes/m/example-docker-buildx-go/sha256:2ebebdde436cbbfea50bf5a4eb20b673029dbe7a68577b4fcf42aec122b5988a
/data/keppel/ruilopes/b/7e95c2bfb24e36d0bea10e05f985aa944647a1d3c6917427939d8b4556449732
/data/keppel/ruilopes/b/192054e7ff72dd5751e12a8b0049ce72709fe182ec081188b53597a3235b36b0
/data/keppel/ruilopes/b/5cfe44bca4a32d425fb1ae6171d62b46903abaa6a679d37272c890bb72cbd18b
```

Execute the image:

```bash
docker rmi "$image"
docker system prune --all --force
docker run --rm "$image"
```

Delete the image:

```bash
crane delete --insecure "$image"
```

Destroy the environment (and all data) with:

```bash
docker compose down --volumes --remove-orphans --timeout=0
```

### Example commands

List all the container images:

```bash
crane catalog --insecure keppel.test:9006 | while read name; do
  crane ls --insecure "keppel.test:9006/$name" | while read tag; do
    echo "keppel.test:9006/$name:$tag"
  done
done
```

Execute some example commands in the `keppel` database:

```bash
docker compose exec --user postgres postgres psql keppel <<'EOF'
\d
\d quotas
\d accounts
\d repos
\d tags
\d manifests
select * from quotas;
select * from accounts;
select * from repos;
select * from tags;
select * from manifests;
EOF
```

Execute some example keppel requests:

```bash
http get keppel.test:9006/keppel/v1
http get keppel.test:9006/keppel/v1/accounts
http get keppel.test:9006/keppel/v1/accounts/keppel
http get keppel.test:9006/keppel/v1/accounts/keppel/repositories
```
