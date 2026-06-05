use std::fs;

use skill_importer::{
    AgentEnablement, DiscoveryRoots, ImportActionKind, ImportError, ImportUrlRequest, SkillSource,
    SkillUrlFetchError, SkillUrlFetcher, discover_skills, import_url_skill,
};

#[test]
fn importing_url_fetches_markdown_stores_manifest_and_inventory_entry() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = DiscoveryRoots {
        canonical_root: temp.path().join("canonical"),
        imports_root: temp.path().join("imports"),
        claude_code_root: temp.path().join("claude"),
        codex_root: temp.path().join("codex"),
    };
    let fetcher = StaticFetcher {
        markdown: r#"---
name: url-helper
description: Imported from a direct URL.
---

# URL Helper
"#,
    };

    let import = import_url_skill(
        &roots,
        ImportUrlRequest {
            url: "https://example.test/url-helper.md",
        },
        &fetcher,
    )
    .expect("import succeeds");

    assert_eq!(import.skill_name, "url-helper");
    assert_eq!(
        import.manifest.source_type,
        skill_importer::ImportSourceType::Url
    );
    assert_eq!(
        import.manifest.content_hash,
        "sha256:941e20dc679905a61b509a0dc9d6f3f1c05a7a1c4233b9ae873cadd29c085f9f"
    );
    assert_eq!(
        import.manifest.source_location.as_deref(),
        Some("https://example.test/url-helper.md")
    );
    assert_eq!(import.actions.len(), 3);
    assert!(
        import
            .actions
            .iter()
            .any(|action| action.action == ImportActionKind::CreateDirectory)
    );
    assert!(import.actions.iter().any(|action| {
        action.action == ImportActionKind::WriteSkill
            && action
                .path
                .ends_with(std::path::Path::new("url-helper").join("SKILL.md"))
    }));
    assert!(import.actions.iter().any(|action| {
        action.action == ImportActionKind::WriteManifest
            && action
                .path
                .ends_with(std::path::Path::new("url-helper").join("import.json"))
    }));
    assert_eq!(
        fs::read_to_string(roots.imports_root.join("url-helper").join("SKILL.md"))
            .expect("stored skill"),
        fetcher.markdown
    );

    let manifest: serde_json::Value = serde_json::from_str(
        &fs::read_to_string(roots.imports_root.join("url-helper").join("import.json"))
            .expect("stored manifest"),
    )
    .expect("manifest json");
    assert_eq!(manifest["source_type"], "url");
    assert_eq!(
        manifest["source_location"],
        "https://example.test/url-helper.md"
    );
    assert_eq!(manifest["promoted"], false);

    let inventory = discover_skills(&roots).expect("discovery succeeds");
    assert_eq!(inventory.skills.len(), 1);
    assert_eq!(inventory.skills[0].name, "url-helper");
    assert_eq!(inventory.skills[0].source, SkillSource::Imported);
    assert_eq!(inventory.skills[0].enablement, AgentEnablement::Neither);
}

struct StaticFetcher {
    markdown: &'static str,
}

impl SkillUrlFetcher for StaticFetcher {
    fn fetch_skill_markdown(&self, _url: &str) -> Result<String, SkillUrlFetchError> {
        Ok(self.markdown.to_string())
    }
}

#[test]
fn url_import_reuses_markdown_validation_without_partial_storage() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = DiscoveryRoots {
        canonical_root: temp.path().join("canonical"),
        imports_root: temp.path().join("imports"),
        claude_code_root: temp.path().join("claude"),
        codex_root: temp.path().join("codex"),
    };
    let fetcher = StaticFetcher {
        markdown: r#"---
name: missing-description
---
"#,
    };

    let error = import_url_skill(
        &roots,
        ImportUrlRequest {
            url: "https://example.test/invalid.md",
        },
        &fetcher,
    )
    .expect_err("import fails");

    match error {
        ImportError::Validation(error) => {
            assert_eq!(error.field, "description");
            assert!(
                error.message.contains("missing"),
                "message should explain the validation failure: {}",
                error.message
            );
        }
        error => panic!("unexpected error: {error}"),
    }

    assert!(
        !roots.imports_root.exists(),
        "invalid fetched Markdown should not create storage"
    );
}

#[test]
fn url_import_reports_fetch_failures_without_partial_storage() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = DiscoveryRoots {
        canonical_root: temp.path().join("canonical"),
        imports_root: temp.path().join("imports"),
        claude_code_root: temp.path().join("claude"),
        codex_root: temp.path().join("codex"),
    };

    let error = import_url_skill(
        &roots,
        ImportUrlRequest {
            url: "https://example.test/missing.md",
        },
        &FailingFetcher,
    )
    .expect_err("import fails");

    match error {
        ImportError::Fetch { url, message } => {
            assert_eq!(url, "https://example.test/missing.md");
            assert!(
                message.contains("404"),
                "message should explain the fetch failure: {message}"
            );
        }
        error => panic!("unexpected error: {error}"),
    }

    assert!(
        !roots.imports_root.exists(),
        "failed fetches should not create storage"
    );
}

#[test]
fn url_import_reuses_canonical_and_import_collision_behavior() {
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
        let markdown = format!(
            r#"---
name: {name}
description: New URL version.
---
"#
        );
        let fetcher = OwnedFetcher { markdown };

        let error = import_url_skill(
            &roots,
            ImportUrlRequest {
                url: "https://example.test/helper.md",
            },
            &fetcher,
        )
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

struct OwnedFetcher {
    markdown: String,
}

impl SkillUrlFetcher for OwnedFetcher {
    fn fetch_skill_markdown(&self, _url: &str) -> Result<String, SkillUrlFetchError> {
        Ok(self.markdown.clone())
    }
}

struct FailingFetcher;

impl SkillUrlFetcher for FailingFetcher {
    fn fetch_skill_markdown(&self, _url: &str) -> Result<String, SkillUrlFetchError> {
        Err(SkillUrlFetchError {
            message: "HTTP 404".to_string(),
        })
    }
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
