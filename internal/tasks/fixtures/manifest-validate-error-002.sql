INSERT INTO accounts (name, auth_tenant_id, upstream_peer_hostname, required_labels, metadata_json, next_blob_sweep_at, next_storage_sweep_at, next_federation_announcement_at, in_maintenance, external_peer_url, external_peer_username, external_peer_password, platform_filter, gc_policies_json) VALUES ('test1', 'test1authtenant', '', '', '', NULL, NULL, NULL, FALSE, '', '', '', '', '[]');

INSERT INTO blob_mounts (blob_id, repo_id, can_be_deleted_at) VALUES (1, 1, NULL);

INSERT INTO blobs (id, account_name, digest, size_bytes, storage_id, pushed_at, validated_at, validation_error_message, can_be_deleted_at, media_type, blocks_vuln_scanning) VALUES (1, 'test1', 'sha256:712dfd307e9f735a037e1391f16c8747e7fb0d1318851e32591b51a6bc600c2d', 1102, '6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b', 133200, 133200, '', NULL, 'application/vnd.docker.container.image.v1+json', NULL);

INSERT INTO manifest_blob_refs (repo_id, digest, blob_id) VALUES (1, 'sha256:8a9217f1887083297faf37cb2c1808f71289f0cd722d6e5157a07be1c362945f', 1);

INSERT INTO manifest_contents (repo_id, digest, content) VALUES (1, 'sha256:8a9217f1887083297faf37cb2c1808f71289f0cd722d6e5157a07be1c362945f', '{"config":{"digest":"sha256:712dfd307e9f735a037e1391f16c8747e7fb0d1318851e32591b51a6bc600c2d","mediaType":"application/vnd.docker.container.image.v1+json","size":1102},"layers":[],"mediaType":"application/vnd.docker.distribution.manifest.v2+json","schemaVersion":2}');

INSERT INTO manifests (repo_id, digest, media_type, size_bytes, pushed_at, validated_at, validation_error_message, last_pulled_at, next_vuln_check_at, vuln_status, vuln_scan_error, labels_json, gc_status_json, min_layer_created_at, max_layer_created_at) VALUES (1, 'sha256:8a9217f1887083297faf37cb2c1808f71289f0cd722d6e5157a07be1c362945f', 'application/vnd.docker.distribution.manifest.v2+json', 1367, 3600, 262800, '', NULL, NULL, 'Pending', '', '', '', NULL, NULL);

INSERT INTO quotas (auth_tenant_id, manifests) VALUES ('test1authtenant', 100);

INSERT INTO repos (id, account_name, name, next_blob_mount_sweep_at, next_manifest_sync_at, next_gc_at) VALUES (1, 'test1', 'foo', NULL, NULL, NULL);
