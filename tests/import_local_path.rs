use std::fs;
use std::os::unix::fs as unix_fs;

use skill_importer::{
    AgentEnablement, DiscoveryRoots, ImportActionKind, ImportError, ImportLocalPathRequest,
    SkillSource, discover_skills, import_local_path_skill,
};

#[test]
fn importing_local_skill_directory_preserves_supporting_files_and_inventory_entry() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = DiscoveryRoots {
        canonical_root: temp.path().join("canonical"),
        imports_root: temp.path().join("imports"),
        claude_code_root: temp.path().join("claude"),
        codex_root: temp.path().join("codex"),
    };
    let source = temp.path().join("source").join("local-helper");

    fs::create_dir_all(source.join("scripts")).expect("support dir");
    fs::write(
        source.join("SKILL.md"),
        r#"---
name: local-helper
description: Imported from a local directory.
---

# Local Helper
"#,
    )
    .expect("skill file");
    fs::write(
        source.join("scripts").join("run.sh"),
        "#!/bin/sh\necho local\n",
    )
    .expect("supporting file");

    let import = import_local_path_skill(&roots, ImportLocalPathRequest { path: &source })
        .expect("import succeeds");

    assert_eq!(import.skill_name, "local-helper");
    assert_eq!(
        import.manifest.source_type,
        skill_importer::ImportSourceType::LocalPath
    );
    assert_eq!(
        import.manifest.source_location.as_deref(),
        Some(source.to_str().expect("source path"))
    );
    assert_eq!(
        fs::read_to_string(roots.imports_root.join("local-helper").join("SKILL.md"))
            .expect("stored skill"),
        fs::read_to_string(source.join("SKILL.md")).expect("source skill")
    );
    assert_eq!(
        fs::read_to_string(
            roots
                .imports_root
                .join("local-helper")
                .join("scripts")
                .join("run.sh")
        )
        .expect("stored support file"),
        "#!/bin/sh\necho local\n"
    );
    assert!(import.actions.iter().any(|action| {
        action.action == ImportActionKind::CopyFile
            && action
                .path
                .ends_with(std::path::Path::new("scripts").join("run.sh"))
    }));

    let manifest: serde_json::Value = serde_json::from_str(
        &fs::read_to_string(roots.imports_root.join("local-helper").join("import.json"))
            .expect("stored manifest"),
    )
    .expect("manifest json");
    assert_eq!(manifest["source_type"], "local_path");
    assert_eq!(
        manifest["source_location"],
        source.to_str().expect("source path")
    );
    assert_eq!(manifest["promoted"], false);

    let inventory = discover_skills(&roots).expect("discovery succeeds");
    assert_eq!(inventory.skills.len(), 1);
    assert_eq!(inventory.skills[0].name, "local-helper");
    assert_eq!(inventory.skills[0].source, SkillSource::Imported);
    assert_eq!(inventory.skills[0].enablement, AgentEnablement::Neither);
}

#[test]
fn importing_local_markdown_file_stores_it_as_managed_skill_file() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = DiscoveryRoots {
        canonical_root: temp.path().join("canonical"),
        imports_root: temp.path().join("imports"),
        claude_code_root: temp.path().join("claude"),
        codex_root: temp.path().join("codex"),
    };
    let source = temp.path().join("downloaded-helper.md");
    let markdown = r#"---
name: downloaded-helper
description: Imported from a local file.
---

# Downloaded Helper
"#;
    fs::write(&source, markdown).expect("source markdown");

    let import = import_local_path_skill(&roots, ImportLocalPathRequest { path: &source })
        .expect("import succeeds");

    assert_eq!(import.skill_name, "downloaded-helper");
    assert_eq!(
        import.manifest.source_type,
        skill_importer::ImportSourceType::LocalPath
    );
    assert_eq!(
        import.manifest.source_location.as_deref(),
        Some(source.to_str().expect("source path"))
    );
    assert_eq!(
        fs::read_to_string(
            roots
                .imports_root
                .join("downloaded-helper")
                .join("SKILL.md")
        )
        .expect("stored markdown"),
        markdown
    );

    let inventory = discover_skills(&roots).expect("discovery succeeds");
    assert_eq!(inventory.skills.len(), 1);
    assert_eq!(inventory.skills[0].name, "downloaded-helper");
    assert_eq!(inventory.skills[0].source, SkillSource::Imported);
}

#[test]
fn local_path_import_reports_invalid_sources_without_partial_storage() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = DiscoveryRoots {
        canonical_root: temp.path().join("canonical"),
        imports_root: temp.path().join("imports"),
        claude_code_root: temp.path().join("claude"),
        codex_root: temp.path().join("codex"),
    };
    let source = temp.path().join("not-a-skill");
    fs::create_dir_all(&source).expect("source dir");

    let error = import_local_path_skill(&roots, ImportLocalPathRequest { path: &source })
        .expect_err("import fails");

    match error {
        ImportError::InvalidSource { path, message } => {
            assert_eq!(path, source);
            assert!(
                message.contains("SKILL.md"),
                "message should name the missing skill file: {message}"
            );
        }
        error => panic!("unexpected error: {error}"),
    }

    assert!(
        !roots.imports_root.exists(),
        "invalid local paths should not create storage"
    );
}

#[test]
fn local_path_import_reuses_canonical_and_import_collision_behavior() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = DiscoveryRoots {
        canonical_root: temp.path().join("canonical"),
        imports_root: temp.path().join("imports"),
        claude_code_root: temp.path().join("claude"),
        codex_root: temp.path().join("codex"),
    };
    write_skill(&roots.canonical_root, "shared-name", "Already canonical.");
    write_skill(&roots.imports_root, "already-imported", "Already imported.");

    for (name, expected_path) in [
        ("shared-name", roots.canonical_root.join("shared-name")),
        (
            "already-imported",
            fs::canonicalize(&roots.imports_root)
                .expect("canonical imports root")
                .join("already-imported"),
        ),
    ] {
        let source = temp.path().join("source").join(name);
        write_skill(
            temp.path().join("source").as_path(),
            name,
            "New local version.",
        );

        let error = import_local_path_skill(&roots, ImportLocalPathRequest { path: &source })
            .expect_err("import fails");

        match error {
            ImportError::Collision {
                name: actual_name,
                path,
            } => {
                assert_eq!(actual_name, name);
                assert_eq!(path, expected_path);
            }
            error => panic!("unexpected error: {error}"),
        }
    }
}

#[test]
fn local_directory_content_hash_includes_supporting_files() {
    let temp = tempfile::tempdir().expect("tempdir");
    let first_roots = DiscoveryRoots {
        canonical_root: temp.path().join("first-canonical"),
        imports_root: temp.path().join("first-imports"),
        claude_code_root: temp.path().join("first-claude"),
        codex_root: temp.path().join("first-codex"),
    };
    let second_roots = DiscoveryRoots {
        canonical_root: temp.path().join("second-canonical"),
        imports_root: temp.path().join("second-imports"),
        claude_code_root: temp.path().join("second-claude"),
        codex_root: temp.path().join("second-codex"),
    };
    let first_source = write_skill(
        &temp.path().join("first-source"),
        "hash-helper",
        "Hash includes support files.",
    );
    let second_source = write_skill(
        &temp.path().join("second-source"),
        "hash-helper",
        "Hash includes support files.",
    );
    fs::create_dir_all(first_source.join("references")).expect("first support dir");
    fs::create_dir_all(second_source.join("references")).expect("second support dir");
    fs::write(first_source.join("references").join("notes.md"), "one\n")
        .expect("first support file");
    fs::write(second_source.join("references").join("notes.md"), "two\n")
        .expect("second support file");

    let first = import_local_path_skill(
        &first_roots,
        ImportLocalPathRequest {
            path: &first_source,
        },
    )
    .expect("first import succeeds");
    let second = import_local_path_skill(
        &second_roots,
        ImportLocalPathRequest {
            path: &second_source,
        },
    )
    .expect("second import succeeds");

    assert_ne!(first.manifest.content_hash, second.manifest.content_hash);
}

#[test]
fn local_directory_import_rejects_unsupported_supporting_entries_without_partial_storage() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = DiscoveryRoots {
        canonical_root: temp.path().join("canonical"),
        imports_root: temp.path().join("imports"),
        claude_code_root: temp.path().join("claude"),
        codex_root: temp.path().join("codex"),
    };
    let source = write_skill(
        &temp.path().join("source"),
        "symlink-helper",
        "Contains an unsupported symlink.",
    );
    fs::write(temp.path().join("shared.txt"), "shared\n").expect("shared target");
    unix_fs::symlink(
        temp.path().join("shared.txt"),
        source.join("linked-support.txt"),
    )
    .expect("support symlink");

    let error = import_local_path_skill(&roots, ImportLocalPathRequest { path: &source })
        .expect_err("import fails");

    match error {
        ImportError::InvalidSource { path, message } => {
            assert_eq!(path, source.join("linked-support.txt"));
            assert!(
                message.contains("unsupported"),
                "message should explain the unsupported entry: {message}"
            );
        }
        error => panic!("unexpected error: {error}"),
    }

    assert!(
        !roots.imports_root.exists(),
        "unsupported entries should be rejected before creating import storage"
    );
}

#[test]
fn local_directory_import_rejects_top_level_import_manifest_support_file() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = DiscoveryRoots {
        canonical_root: temp.path().join("canonical"),
        imports_root: temp.path().join("imports"),
        claude_code_root: temp.path().join("claude"),
        codex_root: temp.path().join("codex"),
    };
    let source = write_skill(
        &temp.path().join("source"),
        "manifest-file-helper",
        "Contains a reserved importer manifest filename.",
    );
    fs::write(source.join("import.json"), r#"{"source":"support"}"#).expect("support manifest");

    let error = import_local_path_skill(&roots, ImportLocalPathRequest { path: &source })
        .expect_err("import fails");

    match error {
        ImportError::InvalidSource { path, message } => {
            assert_eq!(path, source.join("import.json"));
            assert!(
                message.contains("reserved"),
                "message should explain the reserved manifest filename: {message}"
            );
        }
        error => panic!("unexpected error: {error}"),
    }

    assert!(
        !roots.imports_root.exists(),
        "reserved manifest filename should be rejected before creating import storage"
    );
}

#[test]
fn local_directory_import_rejects_import_storage_inside_source() {
    let temp = tempfile::tempdir().expect("tempdir");
    let source = write_skill(
        &temp.path().join("source"),
        "self-copy-helper",
        "Would contain its own import destination.",
    );
    let imports_root = source.join(".skill-importer").join("imports");
    let roots = DiscoveryRoots {
        canonical_root: temp.path().join("canonical"),
        imports_root: imports_root.clone(),
        claude_code_root: temp.path().join("claude"),
        codex_root: temp.path().join("codex"),
    };

    let error = import_local_path_skill(&roots, ImportLocalPathRequest { path: &source })
        .expect_err("import fails");

    match error {
        ImportError::InvalidSource { path, message } => {
            assert_eq!(
                path,
                fs::canonicalize(&source)
                    .expect("canonical source")
                    .join(".skill-importer")
                    .join("imports")
            );
            assert!(
                message.contains("inside"),
                "message should explain the recursive source/storage relationship: {message}"
            );
        }
        error => panic!("unexpected error: {error}"),
    }

    assert!(
        !roots.imports_root.exists(),
        "self-copy rejection should happen before creating import storage"
    );
}

#[test]
fn local_directory_import_rejects_import_storage_inside_symlinked_source() {
    let temp = tempfile::tempdir().expect("tempdir");
    let real_source = write_skill(
        &temp.path().join("real-source"),
        "symlinked-self-copy-helper",
        "Would contain its own real import destination.",
    );
    let linked_source = temp.path().join("linked-source");
    unix_fs::symlink(&real_source, &linked_source).expect("source symlink");
    let imports_root = real_source.join(".skill-importer").join("imports");
    let roots = DiscoveryRoots {
        canonical_root: temp.path().join("canonical"),
        imports_root: imports_root.clone(),
        claude_code_root: temp.path().join("claude"),
        codex_root: temp.path().join("codex"),
    };

    let error = import_local_path_skill(
        &roots,
        ImportLocalPathRequest {
            path: &linked_source,
        },
    )
    .expect_err("import fails");

    match error {
        ImportError::InvalidSource { path, message } => {
            assert_eq!(
                path,
                fs::canonicalize(&real_source)
                    .expect("canonical real source")
                    .join(".skill-importer")
                    .join("imports")
            );
            assert!(
                message.contains("inside"),
                "message should explain the recursive source/storage relationship: {message}"
            );
        }
        error => panic!("unexpected error: {error}"),
    }

    assert!(
        !roots.imports_root.exists(),
        "realpath self-copy rejection should happen before creating import storage"
    );
}

#[test]
fn local_directory_import_rejects_import_storage_inside_symlink_parent_source() {
    let temp = tempfile::tempdir().expect("tempdir");
    let real_source = write_skill(
        &temp.path().join("source"),
        "parent-symlink-self-copy-helper",
        "Would hide its import destination behind a symlink parent.",
    );
    fs::create_dir_all(real_source.join("nested")).expect("nested source dir");
    let linked_nested = temp.path().join("source-link");
    unix_fs::symlink(real_source.join("nested"), &linked_nested).expect("nested source symlink");
    let imports_root = linked_nested
        .join("..")
        .join(".skill-importer")
        .join("imports");
    let roots = DiscoveryRoots {
        canonical_root: temp.path().join("canonical"),
        imports_root: imports_root.clone(),
        claude_code_root: temp.path().join("claude"),
        codex_root: temp.path().join("codex"),
    };

    let error = import_local_path_skill(&roots, ImportLocalPathRequest { path: &real_source })
        .expect_err("import fails");

    match error {
        ImportError::InvalidSource { path, message } => {
            assert_eq!(
                path,
                fs::canonicalize(&real_source)
                    .expect("canonical real source")
                    .join(".skill-importer")
                    .join("imports")
            );
            assert!(
                message.contains("inside"),
                "message should explain the recursive source/storage relationship: {message}"
            );
        }
        error => panic!("unexpected error: {error}"),
    }

    assert!(
        !real_source.join(".skill-importer").join("imports").exists(),
        "symlink-parent self-copy rejection should happen before creating import storage"
    );
}

#[test]
fn local_directory_import_uses_sanitized_imports_root_for_storage() {
    let temp = tempfile::tempdir().expect("tempdir");
    let source = write_skill(
        &temp.path().join("source"),
        "sanitized-import-root-helper",
        "Storage should not create intermediate directories inside the source.",
    );
    let imports_root = source
        .join("missing-intermediate")
        .join("..")
        .join("..")
        .join("imports");
    let roots = DiscoveryRoots {
        canonical_root: temp.path().join("canonical"),
        imports_root,
        claude_code_root: temp.path().join("claude"),
        codex_root: temp.path().join("codex"),
    };

    let import = import_local_path_skill(&roots, ImportLocalPathRequest { path: &source })
        .expect("import succeeds");

    assert_eq!(
        import.skill_path,
        fs::canonicalize(source.parent().expect("source parent"))
            .expect("canonical source parent")
            .join("imports")
            .join("sanitized-import-root-helper")
    );
    assert!(
        !source.join("missing-intermediate").exists(),
        "sanitized storage path should not create intermediate directories inside the source"
    );
    assert!(
        source
            .parent()
            .expect("source parent")
            .join("imports")
            .join("sanitized-import-root-helper")
            .join("SKILL.md")
            .exists()
    );

    let inventory = discover_skills(&roots).expect("discovery succeeds");
    assert_eq!(inventory.skills.len(), 1);
    assert_eq!(inventory.skills[0].name, "sanitized-import-root-helper");
    assert_eq!(inventory.skills[0].source, SkillSource::Imported);
}

#[test]
fn local_directory_content_hash_includes_empty_support_directories() {
    let temp = tempfile::tempdir().expect("tempdir");
    let first_roots = DiscoveryRoots {
        canonical_root: temp.path().join("first-canonical"),
        imports_root: temp.path().join("first-imports"),
        claude_code_root: temp.path().join("first-claude"),
        codex_root: temp.path().join("first-codex"),
    };
    let second_roots = DiscoveryRoots {
        canonical_root: temp.path().join("second-canonical"),
        imports_root: temp.path().join("second-imports"),
        claude_code_root: temp.path().join("second-claude"),
        codex_root: temp.path().join("second-codex"),
    };
    let first_source = write_skill(
        &temp.path().join("first-empty-dir-source"),
        "empty-dir-hash-helper",
        "Hash includes empty support directories.",
    );
    let second_source = write_skill(
        &temp.path().join("second-empty-dir-source"),
        "empty-dir-hash-helper",
        "Hash includes empty support directories.",
    );
    fs::create_dir_all(second_source.join("templates").join("empty")).expect("empty support dir");

    let first = import_local_path_skill(
        &first_roots,
        ImportLocalPathRequest {
            path: &first_source,
        },
    )
    .expect("first import succeeds");
    let second = import_local_path_skill(
        &second_roots,
        ImportLocalPathRequest {
            path: &second_source,
        },
    )
    .expect("second import succeeds");

    assert_ne!(first.manifest.content_hash, second.manifest.content_hash);
}

#[test]
fn local_directory_content_hash_uses_unambiguous_binary_framing() {
    let temp = tempfile::tempdir().expect("tempdir");
    let first_roots = DiscoveryRoots {
        canonical_root: temp.path().join("first-canonical"),
        imports_root: temp.path().join("first-imports"),
        claude_code_root: temp.path().join("first-claude"),
        codex_root: temp.path().join("first-codex"),
    };
    let second_roots = DiscoveryRoots {
        canonical_root: temp.path().join("second-canonical"),
        imports_root: temp.path().join("second-imports"),
        claude_code_root: temp.path().join("second-claude"),
        codex_root: temp.path().join("second-codex"),
    };
    let first_source = write_skill(
        &temp.path().join("first-binary-source"),
        "binary-hash-helper",
        "Hash frames binary support files.",
    );
    let second_source = write_skill(
        &temp.path().join("second-binary-source"),
        "binary-hash-helper",
        "Hash frames binary support files.",
    );
    fs::write(first_source.join("a"), b"x").expect("first a");
    fs::write(first_source.join("b"), b"y").expect("first b");
    fs::write(second_source.join("a"), b"x\0file\0b\0y").expect("second a");

    let first = import_local_path_skill(
        &first_roots,
        ImportLocalPathRequest {
            path: &first_source,
        },
    )
    .expect("first import succeeds");
    let second = import_local_path_skill(
        &second_roots,
        ImportLocalPathRequest {
            path: &second_source,
        },
    )
    .expect("second import succeeds");

    assert_ne!(first.manifest.content_hash, second.manifest.content_hash);
}

fn write_skill(root: &std::path::Path, name: &str, description: &str) -> std::path::PathBuf {
    let skill_dir = root.join(name);
    fs::create_dir_all(&skill_dir).expect("skill dir");
    fs::write(
        skill_dir.join("SKILL.md"),
        format!(
            r#"---
name: {name}
description: {description}
---
"#
        ),
    )
    .expect("skill file");
    skill_dir
}
