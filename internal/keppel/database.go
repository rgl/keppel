/*******************************************************************************
*
* Copyright 2018-2020 SAP SE
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
	"net/url"

	"github.com/sapcc/go-bits/easypg"
	gorp "gopkg.in/gorp.v2"
)

var sqlMigrations = map[string]string{
	"001_initial.up.sql": `
		CREATE TABLE accounts (
			name                   TEXT NOT NULL PRIMARY KEY,
			auth_tenant_id         TEXT NOT NULL,
			upstream_peer_hostname TEXT NOT NULL DEFAULT ''
		);

		CREATE TABLE rbac_policies (
			account_name     TEXT    NOT NULL REFERENCES accounts ON DELETE CASCADE,
			match_repository TEXT    NOT NULL,
			match_username   TEXT    NOT NULL,
			can_anon_pull    BOOLEAN NOT NULL DEFAULT FALSE,
			can_pull         BOOLEAN NOT NULL DEFAULT FALSE,
			can_push         BOOLEAN NOT NULL DEFAULT FALSE,
			can_delete       BOOLEAN NOT NULL DEFAULT FALSE,
			PRIMARY KEY (account_name, match_repository, match_username)
		);

		CREATE TABLE quotas (
			auth_tenant_id TEXT   NOT NULL PRIMARY KEY,
			manifests      BIGINT NOT NULL
		);

		CREATE TABLE peers (
			hostname                     TEXT        NOT NULL PRIMARY KEY,
			our_password                 TEXT        NOT NULL DEFAULT '',
			their_current_password_hash  TEXT        NOT NULL DEFAULT '',
			their_previous_password_hash TEXT        NOT NULL DEFAULT '',
			last_peered_at               TIMESTAMPTZ DEFAULT NULL
		);

		CREATE TABLE repos (
			id           BIGSERIAL NOT NULL PRIMARY KEY,
			account_name TEXT      NOT NULL REFERENCES accounts ON DELETE CASCADE,
			name         TEXT      NOT NULL
		);

		CREATE TABLE blobs (
			id           BIGSERIAL   NOT NULL PRIMARY KEY,
			account_name TEXT        NOT NULL REFERENCES accounts ON DELETE CASCADE,
			digest       TEXT        NOT NULL,
			size_bytes   BIGINT      NOT NULL,
			storage_id   TEXT        NOT NULL,
			pushed_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (account_name, digest)
		);

		CREATE TABLE blob_mounts (
			blob_id BIGINT NOT NULL REFERENCES blobs ON DELETE CASCADE,
			repo_id BIGINT NOT NULL REFERENCES repos ON DELETE CASCADE,
			UNIQUE (blob_id, repo_id)
		);

		CREATE TABLE uploads (
			repo_id     BIGINT      NOT NULL REFERENCES repos ON DELETE CASCADE,
			uuid        TEXT        NOT NULL,
			storage_id  TEXT        NOT NULL,
			size_bytes  BIGINT      NOT NULL,
			digest      TEXT        NOT NULL,
			num_chunks  INT         NOT NULL,
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (repo_id, uuid)
		);

		CREATE TABLE manifests (
			repo_id    BIGINT      NOT NULL REFERENCES repos ON DELETE CASCADE,
			digest     TEXT        NOT NULL,
			media_type TEXT        NOT NULL,
			size_bytes BIGINT      NOT NULL,
			pushed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (repo_id, digest)
		);

		CREATE TABLE tags (
			repo_id    BIGINT      NOT NULL REFERENCES repos ON DELETE CASCADE,
			name       TEXT        NOT NULL,
			digest     TEXT        NOT NULL,
			pushed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (repo_id, name),
			FOREIGN KEY (repo_id, digest) REFERENCES manifests ON DELETE CASCADE
		);

		CREATE TABLE pending_blobs (
			repo_id BIGINT      NOT NULL REFERENCES repos ON DELETE CASCADE,
			digest  TEXT        NOT NULL,
			reason  TEXT        NOT NULL,
			since   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (repo_id, digest)
		);
	`,
	"001_initial.down.sql": `
		DROP TABLE pending_blobs;
		DROP TABLE tags;
		DROP TABLE manifests;
		DROP TABLE uploads;
		DROP TABLE blob_mounts;
		DROP TABLE blobs;
		DROP TABLE repos;
		DROP TABLE peers;
		DROP TABLE quotas;
		DROP TABLE rbac_policies;
		DROP TABLE accounts;
	`,
	"002_add_account_required_labels.up.sql": `
		ALTER TABLE accounts ADD column required_labels TEXT NOT NULL DEFAULT '';
	`,
	"002_add_account_required_labels.down.sql": `
		ALTER TABLE accounts DROP column required_labels;
	`,
	"003_add_repos_uniqueness_constraint.up.sql": `
		ALTER TABLE repos ADD CONSTRAINT repos_account_name_name_key UNIQUE (account_name, name);
	`,
	"003_add_repos_uniqueness_constraint.down.sql": `
		ALTER TABLE repos DROP CONSTRAINT repos_account_name_name_key;
	`,
	"004_add_manifest_subreferences.up.sql": `
		CREATE TABLE manifest_blob_refs (
			repo_id BIGINT NOT NULL,
			digest  TEXT   NOT NULL,
			blob_id BIGINT NOT NULL       REFERENCES blobs ON DELETE RESTRICT,
			FOREIGN KEY (repo_id, digest) REFERENCES manifests ON DELETE CASCADE,
			UNIQUE (repo_id, digest, blob_id)
		);
		CREATE TABLE manifest_manifest_refs (
			repo_id       BIGINT NOT NULL,
			parent_digest TEXT   NOT NULL,
			child_digest  TEXT   NOT NULL,
			FOREIGN KEY (repo_id, parent_digest) REFERENCES manifests (repo_id, digest) ON DELETE CASCADE,
			FOREIGN KEY (repo_id, child_digest)  REFERENCES manifests (repo_id, digest) ON DELETE RESTRICT,
			UNIQUE (repo_id, parent_digest, child_digest)
		);
		ALTER TABLE manifests
			ADD COLUMN validated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			ADD COLUMN validation_error_message TEXT        NOT NULL DEFAULT '';
		ALTER TABLE blobs
			ADD COLUMN validated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			ADD COLUMN validation_error_message TEXT        NOT NULL DEFAULT '';
	`,
	"004_add_manifest_subreferences.down.sql": `
		DROP TABLE manifest_blob_refs;
		DROP TABLE manifest_manifest_refs;
		ALTER TABLE manifests
			DROP COLUMN validated_at,
			DROP COLUMN validation_error_message;
		ALTER TABLE blobs
			DROP COLUMN validated_at,
			DROP COLUMN validation_error_message;
	`,
	"005_rebase_pending_blobs_on_accounts.up.sql": `
		DROP TABLE pending_blobs;
		CREATE TABLE pending_blobs (
			account_name TEXT        NOT NULL REFERENCES accounts ON DELETE CASCADE,
			digest       TEXT        NOT NULL,
			reason       TEXT        NOT NULL,
			since        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (account_name, digest)
		);
	`,
	"005_rebase_pending_blobs_on_accounts.down.sql": `
		DROP TABLE pending_blobs;
		CREATE TABLE pending_blobs (
			repo_id BIGINT      NOT NULL REFERENCES repos ON DELETE CASCADE,
			digest  TEXT        NOT NULL,
			reason  TEXT        NOT NULL,
			since   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (repo_id, digest)
		);
	`,
	"006_forbid_blob_mount_deletion_while_referenced.up.sql": `
		ALTER TABLE manifest_blob_refs
			DROP CONSTRAINT manifest_blob_refs_blob_id_fkey,
			ADD FOREIGN KEY (blob_id, repo_id) REFERENCES blob_mounts (blob_id, repo_id) ON DELETE RESTRICT;
	`,
	"006_forbid_blob_mount_deletion_while_referenced.down.sql": `
		ALTER TABLE manifest_blob_refs
			DROP CONSTRAINT manifest_blob_refs_blob_id_repo_id_fkey,
			ADD FOREIGN KEY blob_id REFERENCES blobs (id) ON DELETE RESTRICT;
	`,
	"007_add_garbage_collection_timestamps.up.sql": `
		ALTER TABLE accounts
			ADD COLUMN blobs_sweeped_at   TIMESTAMPTZ DEFAULT NULL,
			ADD COLUMN storage_sweeped_at TIMESTAMPTZ DEFAULT NULL;
		ALTER TABLE repos       ADD COLUMN blob_mounts_sweeped_at TIMESTAMPTZ DEFAULT NULL;
		ALTER TABLE blobs       ADD COLUMN marked_for_deletion_at TIMESTAMPTZ DEFAULT NULL;
		ALTER TABLE blob_mounts ADD COLUMN marked_for_deletion_at TIMESTAMPTZ DEFAULT NULL;
	`,
	"007_add_garbage_collection_timestamps.down.sql": `
		ALTER TABLE accounts
			DROP COLUMN blobs_sweeped_at,
			DROP COLUMN storage_sweeped_at;
		ALTER TABLE repos       DROP COLUMN blob_mounts_sweeped_at;
		ALTER TABLE blobs       DROP COLUMN marked_for_deletion_at;
		ALTER TABLE blob_mounts DROP COLUMN marked_for_deletion_at;
	`,
	"008_add_repos_manifests_synced_at.up.sql": `
		ALTER TABLE repos ADD COLUMN manifests_synced_at TIMESTAMPTZ DEFAULT NULL;
	`,
	"008_add_repos_manifests_synced_at.down.sql": `
		ALTER TABLE repos DROP COLUMN manifests_synced_at;
	`,
	"009_add_tables_for_storage_sweep.up.sql": `
		CREATE TABLE unknown_blobs (
			account_name           TEXT        NOT NULL REFERENCES accounts ON DELETE CASCADE,
			storage_id             TEXT        NOT NULL,
			marked_for_deletion_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (account_name, storage_id)
		);
		CREATE TABLE unknown_manifests (
			account_name           TEXT        NOT NULL REFERENCES accounts ON DELETE CASCADE,
			repo_name              TEXT        NOT NULL,
			digest                 TEXT        NOT NULL,
			marked_for_deletion_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (account_name, repo_name, digest)
		);
	`,
	"009_add_tables_for_storage_sweep.down.sql": `
		DROP TABLE unknown_blobs;
		DROP TABLE unknown_manifests;
	`,
	"010_add_account_metadata.up.sql": `
		ALTER TABLE accounts ADD COLUMN metadata_json TEXT NOT NULL DEFAULT '';
	`,
	"010_add_account_metadata.down.sql": `
		ALTER TABLE accounts DROP COLUMN metadata_json;
	`,
	"011_add_account_announced_to_federation_at.up.sql": `
		ALTER TABLE accounts ADD COLUMN announced_to_federation_at TIMESTAMPTZ DEFAULT NULL;
	`,
	"011_add_account_announced_to_federation_at.down.sql": `
		ALTER TABLE accounts DROP COLUMN announced_to_federation_at;
	`,
	"012_flip_timestamp_semantics.up.sql": `
		ALTER TABLE accounts
			DROP COLUMN blobs_sweeped_at,
			DROP COLUMN storage_sweeped_at,
			DROP COLUMN announced_to_federation_at,
			ADD COLUMN next_blob_sweep_at TIMESTAMPTZ DEFAULT NULL,
			ADD COLUMN next_storage_sweep_at TIMESTAMPTZ DEFAULT NULL,
			ADD COLUMN next_federation_announcement_at TIMESTAMPTZ DEFAULT NULL;
		ALTER TABLE blobs
			DROP COLUMN marked_for_deletion_at,
			ADD COLUMN can_be_deleted_at TIMESTAMPTZ DEFAULT NULL;
		ALTER TABLE blob_mounts
			DROP COLUMN marked_for_deletion_at,
			ADD COLUMN can_be_deleted_at TIMESTAMPTZ DEFAULT NULL;
		ALTER TABLE repos
			DROP COLUMN blob_mounts_sweeped_at,
			DROP COLUMN manifests_synced_at,
			ADD COLUMN next_blob_mount_sweep_at TIMESTAMPTZ DEFAULT NULL,
			ADD COLUMN next_manifest_sync_at TIMESTAMPTZ DEFAULT NULL;
		DELETE FROM unknown_blobs;
		ALTER TABLE unknown_blobs RENAME COLUMN marked_for_deletion_at TO can_be_deleted_at;
		DELETE FROM unknown_manifests;
		ALTER TABLE unknown_manifests RENAME COLUMN marked_for_deletion_at TO can_be_deleted_at;
	`,
	"012_flip_timestamp_semantics.down.sql": `
		ALTER TABLE accounts
			DROP COLUMN next_blob_sweep_at,
			DROP COLUMN next_storage_sweep_at,
			DROP COLUMN next_federation_announcement_at,
			ADD COLUMN blobs_sweeped_at TIMESTAMPTZ DEFAULT NULL,
			ADD COLUMN storage_sweeped_at TIMESTAMPTZ DEFAULT NULL,
			ADD COLUMN announced_to_federation_at TIMESTAMPTZ DEFAULT NULL;
		ALTER TABLE blobs
			DROP COLUMN can_be_deleted_at,
			ADD COLUMN marked_for_deletion_at TIMESTAMPTZ DEFAULT NULL;
		ALTER TABLE blob_mounts
			DROP COLUMN can_be_deleted_at,
			ADD COLUMN marked_for_deletion_at TIMESTAMPTZ DEFAULT NULL;
		ALTER TABLE repos
			DROP COLUMN next_blob_mount_sweep_at,
			DROP COLUMN next_manifest_sync_at,
			ADD COLUMN blob_mounts_sweeped_at TIMESTAMPTZ DEFAULT NULL,
			ADD COLUMN manifests_synced_at TIMESTAMPTZ DEFAULT NULL;
		DELETE FROM unknown_blobs;
		ALTER TABLE unknown_blobs RENAME COLUMN can_be_deleted_at TO marked_for_deletion_at;
		DELETE FROM unknown_manifests;
		ALTER TABLE unknown_manifests RENAME COLUMN can_be_deleted_at TO marked_for_deletion_at;
	`,
	"013_add_account_in_maintenance.up.sql": `
		ALTER TABLE accounts ADD COLUMN in_maintenance BOOLEAN NOT NULL DEFAULT FALSE;
	`,
	"013_add_account_in_maintenance.down.sql": `
		ALTER TABLE accounts DROP COLUMN in_maintenance;
	`,
	"014_add_manifests_last_pulled_at.up.sql": `
		ALTER TABLE manifests ADD COLUMN last_pulled_at TIMESTAMPTZ DEFAULT NULL;
	`,
	"014_add_manifests_last_pulled_at.down.sql": `
		ALTER TABLE manifests DROP COLUMN last_pulled_at;
	`,
	"015_add_tags_last_pulled_at.up.sql": `
		ALTER TABLE tags ADD COLUMN last_pulled_at TIMESTAMPTZ DEFAULT NULL;
	`,
	"015_add_tags_last_pulled_at.down.sql": `
		ALTER TABLE tags DROP COLUMN last_pulled_at;
	`,
	"016_add_account_external_replica_credentials.up.sql": `
		ALTER TABLE accounts
			ADD COLUMN external_peer_url      TEXT DEFAULT NULL,
			ADD COLUMN external_peer_username TEXT DEFAULT NULL,
			ADD COLUMN external_peer_password TEXT DEFAULT NULL;
	`,
	"016_add_account_external_replica_credentials.down.sql": `
		ALTER TABLE accounts
			DROP COLUMN external_peer_url,
			DROP COLUMN external_peer_username,
			DROP COLUMN external_peer_password;
	`,
	"017_fix_datatypes.up.sql": `
		ALTER TABLE accounts
			DROP COLUMN external_peer_url,
			DROP COLUMN external_peer_username,
			DROP COLUMN external_peer_password;
		ALTER TABLE accounts
			ADD COLUMN external_peer_url      TEXT NOT NULL DEFAULT '',
			ADD COLUMN external_peer_username TEXT NOT NULL DEFAULT '',
			ADD COLUMN external_peer_password TEXT NOT NULL DEFAULT '';
	`,
	"017_fix_datatypes.down.sql": `
		ALTER TABLE accounts
			DROP COLUMN external_peer_url,
			DROP COLUMN external_peer_username,
			DROP COLUMN external_peer_password;
		ALTER TABLE accounts
			ADD COLUMN external_peer_url      TEXT DEFAULT NULL,
			ADD COLUMN external_peer_username TEXT DEFAULT NULL,
			ADD COLUMN external_peer_password TEXT DEFAULT NULL;
	`,
	"018_track_vulnerability_status.up.sql": `
		ALTER TABLE manifests
			ADD COLUMN next_vuln_check_at TIMESTAMPTZ DEFAULT NULL,
			ADD COLUMN vuln_status TEXT NOT NULL DEFAULT 'Unknown';
	`,
	"018_track_vulnerability_status.down.sql": `
		ALTER TABLE manifests
			DROP COLUMN next_vuln_check_at,
			DROP COLUMN vuln_status;
	`,
	"019_fix_default_vulnerability_status.up.sql": `
		ALTER TABLE manifests ALTER COLUMN vuln_status SET DEFAULT 'Pending';
		UPDATE manifests SET vuln_status = 'Pending', next_vuln_check_at = NULL WHERE vuln_status = 'Unknown';
	`,
	"019_fix_default_vulnerability_status.down.sql": `
		ALTER TABLE manifests ALTER COLUMN vuln_status SET DEFAULT 'Unknown';
		UPDATE manifests SET vuln_status = 'Unknown', next_vuln_check_at = NULL WHERE vuln_status = 'Pending';
	`,
	"020_add_account_platform_filter.up.sql": `
		ALTER TABLE accounts
			ADD COLUMN platform_filter TEXT NOT NULL DEFAULT '';
	`,
	"020_add_account_platform_filter.down.sql": `
		ALTER TABLE accounts
			DROP COLUMN platform_filter;
	`,
	"021_add_manifest_vuln_scan_error.up.sql": `
		ALTER TABLE manifests
			ADD COLUMN vuln_scan_error TEXT NOT NULL DEFAULT '';
	`,
	"021_add_manifest_vuln_scan_error.down.sql": `
		ALTER TABLE manifests
			DROP COLUMN vuln_scan_error;
	`,
	"022_add_blob_media_type.up.sql": `
		ALTER TABLE blobs
			ADD COLUMN media_type TEXT NOT NULL DEFAULT '';
	`,
	"022_add_blob_media_type.down.sql": `
		ALTER TABLE blobs
			DROP COLUMN media_type;
	`,
	"023_manifest_garbage_collection.up.sql": `
		ALTER TABLE accounts
			ADD COLUMN gc_policies_json TEXT NOT NULL DEFAULT '[]';
		ALTER TABLE repos
			ADD COLUMN next_gc_at TIMESTAMPTZ DEFAULT NULL;
	`,
	"023_manifest_garbage_collection.down.sql": `
		ALTER TABLE accounts
			DROP COLUMN gc_policies_json;
		ALTER TABLE repos
			DROP COLUMN next_gc_at;
	`,
	"024_add_manifest_labels.up.sql": `
		ALTER TABLE manifests
			ADD COLUMN labels_json TEXT NOT NULL DEFAULT '';
	`,
	"024_add_manifest_labels.down.sql": `
		ALTER TABLE manifests
			DROP COLUMN labels_json;
	`,
	"025_add_manifest_contents.up.sql": `
		CREATE TABLE manifest_contents (
			repo_id BIGINT NOT NULL,
			digest  TEXT   NOT NULL,
			content BYTEA  NOT NULL,
			FOREIGN KEY (repo_id, digest) REFERENCES manifests ON DELETE CASCADE,
			UNIQUE (repo_id, digest)
		);
	`,
	"025_add_manifest_contents.down.sql": `
		DROP TABLE manifest_contents;
	`,
	"026_add_manifests_gc_status_json.up.sql": `
		ALTER TABLE manifests ADD COLUMN gc_status_json TEXT NOT NULL DEFAULT '';
	`,
	"026_add_manifests_gc_status_json.down.sql": `
		ALTER TABLE manifests DROP COLUMN gc_status_json;
	`,
	"027_add_rbac_policies_match_cidr.up.sql": `
		ALTER TABLE rbac_policies
			DROP CONSTRAINT rbac_policies_pkey;
		ALTER TABLE rbac_policies
			ADD COLUMN match_cidr TEXT NOT NULL DEFAULT '0.0.0.0/0';
		ALTER TABLE rbac_policies
			ADD PRIMARY KEY (account_name, match_cidr, match_repository, match_username);
	`,
	"027_add_rbac_policies_match_cidr.down.sql": `
		ALTER TABLE rbac_policies
			DROP CONSTRAINT rbac_policies_pkey;
		ALTER TABLE rbac_policies
			DROP COLUMN match_cidr;
		ALTER TABLE rbac_policies
			ADD PRIMARY KEY (account_name, match_repository, match_username);
	`,
	"028_add_rbac_policies_can_anon_first_pull.up.sql": `
		ALTER TABLE rbac_policies
			ADD COLUMN can_anon_first_pull BOOLEAN NOT NULL DEFAULT FALSE;
	`,
	"028_add_rbac_policies_can_anon_first_pull.down.sql": `
		ALTER TABLE rbac_policies
			DROP COLUMN can_anon_first_pull;
	`,
	"029_add_layer_created_at.up.sql": `
		ALTER TABLE manifests
			ADD COLUMN min_layer_created_at TIMESTAMPTZ DEFAULT NULL,
			ADD COLUMN max_layer_created_at TIMESTAMPTZ DEFAULT NULL;
	`,
	"029_add_layer_created_at.down.sql": `
		ALTER TABLE manifests
			DROP COLUMN min_layer_created_at,
			DROP COLUMN max_layer_created_at;
	`,
	"030_add_blobs_blocks_vuln_scanning.up.sql": `
		ALTER TABLE blobs
			ADD COLUMN blocks_vuln_scanning BOOLEAN DEFAULT NULL;
`,
	"030_add_blobs:blocks_vuln_scanning.down.sql": `
		ALTER TABLE blobs
			DROP COLUMN blocks_vuln_scanning ;
`,
}

// DB adds convenience functions on top of gorp.DbMap.
type DB struct {
	gorp.DbMap
}

// SelectBool is analogous to the other SelectFoo() functions from gorp.DbMap
// like SelectFloat, SelectInt, SelectStr, etc.
func (db *DB) SelectBool(query string, args ...interface{}) (bool, error) {
	var result bool
	err := db.QueryRow(query, args...).Scan(&result)
	return result, err
}

// InitDB connects to the Postgres database.
func InitDB(dbURL *url.URL) (*DB, error) {
	db, err := easypg.Connect(easypg.Configuration{
		PostgresURL: dbURL,
		Migrations:  sqlMigrations,
	})
	if err != nil {
		return nil, err
	}
	//ensure that this process does not starve other Keppel processes for DB connections
	db.SetMaxOpenConns(16)

	result := &DB{DbMap: gorp.DbMap{Db: db, Dialect: gorp.PostgresDialect{}}}
	initModels(&result.DbMap)
	return result, nil
}
