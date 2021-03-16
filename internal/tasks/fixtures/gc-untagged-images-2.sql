INSERT INTO accounts (name, auth_tenant_id, upstream_peer_hostname, required_labels, metadata_json, next_blob_sweep_at, next_storage_sweep_at, next_federation_announcement_at, in_maintenance, external_peer_url, external_peer_username, external_peer_password, platform_filter, gc_policies_json) VALUES ('test1', 'test1authtenant', '', '', '', NULL, NULL, NULL, FALSE, '', '', '', '', '[{"match_repository":".*","strategy":"delete_untagged"}]');

INSERT INTO blob_mounts (blob_id, repo_id, can_be_deleted_at) VALUES (1, 1, NULL);
INSERT INTO blob_mounts (blob_id, repo_id, can_be_deleted_at) VALUES (2, 1, NULL);
INSERT INTO blob_mounts (blob_id, repo_id, can_be_deleted_at) VALUES (3, 1, NULL);
INSERT INTO blob_mounts (blob_id, repo_id, can_be_deleted_at) VALUES (4, 1, NULL);

INSERT INTO blobs (id, account_name, digest, size_bytes, storage_id, pushed_at, validated_at, validation_error_message, can_be_deleted_at, media_type) VALUES (1, 'test1', 'sha256:3399f8c47219da521dcabade3b328f086e1aeb57c107e5d5a1ec07f2a4f4b20b', 1257, '3399f8c47219da521dcabade3b328f086e1aeb57c107e5d5a1ec07f2a4f4b20b', 3600, 3600, '', NULL, '');
INSERT INTO blobs (id, account_name, digest, size_bytes, storage_id, pushed_at, validated_at, validation_error_message, can_be_deleted_at, media_type) VALUES (2, 'test1', 'sha256:3d309a098968afe810ee167e0c5b205ef3610829a6d34e0f0ba4ca66756c6f5e', 1048576, '3d309a098968afe810ee167e0c5b205ef3610829a6d34e0f0ba4ca66756c6f5e', 3600, 3600, '', NULL, '');
INSERT INTO blobs (id, account_name, digest, size_bytes, storage_id, pushed_at, validated_at, validation_error_message, can_be_deleted_at, media_type) VALUES (3, 'test1', 'sha256:359aa5408fc03ed0a8c865fcf4e0a04d086c4a2ba202406990afcc5efb05c3d7', 1257, '359aa5408fc03ed0a8c865fcf4e0a04d086c4a2ba202406990afcc5efb05c3d7', 3600, 3600, '', NULL, '');
INSERT INTO blobs (id, account_name, digest, size_bytes, storage_id, pushed_at, validated_at, validation_error_message, can_be_deleted_at, media_type) VALUES (4, 'test1', 'sha256:a718f4a112454b50c8ecd2b0a5b00eb32ee90699593625139cd3fabc97dcce8d', 1048576, 'a718f4a112454b50c8ecd2b0a5b00eb32ee90699593625139cd3fabc97dcce8d', 3600, 3600, '', NULL, '');

INSERT INTO manifest_blob_refs (repo_id, digest, blob_id) VALUES (1, 'sha256:7c5ed02bcdf0dbddf6f1664e01d6a1505c880e296a599371eb919e0e053c0aef', 1);
INSERT INTO manifest_blob_refs (repo_id, digest, blob_id) VALUES (1, 'sha256:7c5ed02bcdf0dbddf6f1664e01d6a1505c880e296a599371eb919e0e053c0aef', 2);

INSERT INTO manifests (repo_id, digest, media_type, size_bytes, pushed_at, validated_at, validation_error_message, last_pulled_at, next_vuln_check_at, vuln_status, vuln_scan_error) VALUES (1, 'sha256:7c5ed02bcdf0dbddf6f1664e01d6a1505c880e296a599371eb919e0e053c0aef', 'application/vnd.docker.distribution.manifest.v2+json', 1050261, 3600, 3600, '', NULL, NULL, 'Pending', '');

INSERT INTO repos (id, account_name, name, next_blob_mount_sweep_at, next_manifest_sync_at, next_gc_at) VALUES (1, 'test1', 'foo', NULL, NULL, 21600);

INSERT INTO tags (repo_id, name, digest, pushed_at, last_pulled_at) VALUES (1, 'first', 'sha256:7c5ed02bcdf0dbddf6f1664e01d6a1505c880e296a599371eb919e0e053c0aef', 3600, NULL);
