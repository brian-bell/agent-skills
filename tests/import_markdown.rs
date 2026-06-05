use std::fs;

use skill_importer::{
    AgentEnablement, DiscoveryRoots, ImportActionKind, ImportError, ImportMarkdownRequest,
    SkillSource, discover_skills, import_markdown_skill,
};

#[test]
fn importing_valid_markdown_stores_manifest_and_appears_in_inventory() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = DiscoveryRoots {
        canonical_root: temp.path().join("canonical"),
        imports_root: temp.path().join("imports"),
        claude_code_root: temp.path().join("claude"),
        codex_root: temp.path().join("codex"),
    };

    let markdown = r#"---
name: pasted-helper
description: Helps pasted imports land safely.
---

# Pasted Helper
"#;

    let import = import_markdown_skill(
        &roots,
        ImportMarkdownRequest {
            markdown,
            source_location: Some("clipboard"),
        },
    )
    .expect("import succeeds");

    assert_eq!(import.skill_name, "pasted-helper");
    assert_eq!(import.actions.len(), 3);
    let write_skill_action = import
        .actions
        .iter()
        .find(|action| action.action == ImportActionKind::WriteSkill)
        .expect("write skill action");
    assert_eq!(
        write_skill_action.path,
        fs::canonicalize(&roots.imports_root)
            .expect("canonical imports root")
            .join("pasted-helper")
            .join("SKILL.md")
    );
    assert_eq!(
        fs::read_to_string(roots.imports_root.join("pasted-helper").join("SKILL.md"))
            .expect("stored skill"),
        markdown
    );

    let manifest: serde_json::Value = serde_json::from_str(
        &fs::read_to_string(roots.imports_root.join("pasted-helper").join("import.json"))
            .expect("stored manifest"),
    )
    .expect("manifest json");
    assert_eq!(manifest["source_type"], "markdown");
    assert_eq!(manifest["source_location"], "clipboard");
    assert_eq!(manifest["promoted"], false);
    assert_eq!(
        manifest["content_hash"],
        "sha256:176dde4267c52602109b1fc7fe30dc368c7c8a02fb0232dd2378941cc56296f3"
    );
    assert!(
        manifest["imported_at"].as_u64().expect("import time") > 0,
        "imported_at should be a Unix timestamp"
    );

    let inventory = discover_skills(&roots).expect("discovery succeeds");
    assert_eq!(inventory.skills.len(), 1);
    let skill = &inventory.skills[0];
    assert_eq!(skill.name, "pasted-helper");
    assert_eq!(
        skill.description.as_deref(),
        Some("Helps pasted imports land safely.")
    );
    assert_eq!(skill.source, SkillSource::Imported);
    assert_eq!(skill.enablement, AgentEnablement::Neither);
}

#[test]
fn markdown_import_requires_description_before_storage() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = DiscoveryRoots {
        canonical_root: temp.path().join("canonical"),
        imports_root: temp.path().join("imports"),
        claude_code_root: temp.path().join("claude"),
        codex_root: temp.path().join("codex"),
    };

    let error = import_markdown_skill(
        &roots,
        ImportMarkdownRequest {
            markdown: r#"---
name: missing-description
---
"#,
            source_location: None,
        },
    )
    .expect_err("import fails");

    match error {
        ImportError::Validation(error) => {
            assert_eq!(error.field, "description");
            assert!(
                error.message.contains("missing"),
                "message: {}",
                error.message
            );
        }
        error => panic!("unexpected error: {error}"),
    }

    assert!(
        !roots.imports_root.exists(),
        "invalid import should not create storage"
    );
}

#[test]
fn markdown_import_names_missing_and_empty_frontmatter_fields() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = DiscoveryRoots {
        canonical_root: temp.path().join("canonical"),
        imports_root: temp.path().join("imports"),
        claude_code_root: temp.path().join("claude"),
        codex_root: temp.path().join("codex"),
    };

    for (markdown, expected_field) in [
        (
            r#"---
description: Missing a name.
---
"#,
            "name",
        ),
        (
            r#"---
name:
description: Empty name.
---
"#,
            "name",
        ),
        (
            r#"---
name: empty-description
description:
---
"#,
            "description",
        ),
        (
            r#"name: no-frontmatter
description: Missing delimiter.
"#,
            "frontmatter",
        ),
        (
            r#"---
name: no-closing-delimiter
description: Missing delimiter.
"#,
            "frontmatter",
        ),
    ] {
        let error = import_markdown_skill(
            &roots,
            ImportMarkdownRequest {
                markdown,
                source_location: None,
            },
        )
        .expect_err("import fails");

        match error {
            ImportError::Validation(error) => assert_eq!(error.field, expected_field),
            error => panic!("unexpected error: {error}"),
        }
    }

    assert!(
        !roots.imports_root.exists(),
        "invalid frontmatter should not create storage"
    );
}

#[test]
fn markdown_import_refuses_canonical_and_import_collisions() {
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
description: New pasted version.
---
"#
        );
        let error = import_markdown_skill(
            &roots,
            ImportMarkdownRequest {
                markdown: &markdown,
                source_location: None,
            },
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

#[test]
fn markdown_import_refuses_frontmatter_name_collisions() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = DiscoveryRoots {
        canonical_root: temp.path().join("canonical"),
        imports_root: temp.path().join("imports"),
        claude_code_root: temp.path().join("claude"),
        codex_root: temp.path().join("codex"),
    };

    let differently_named_dir = roots.canonical_root.join("different-directory");
    fs::create_dir_all(&differently_named_dir).expect("canonical skill dir");
    fs::write(
        differently_named_dir.join("SKILL.md"),
        r#"---
name: semantic-name
description: The frontmatter name is what discovery merges by.
---
"#,
    )
    .expect("canonical skill file");

    let error = import_markdown_skill(
        &roots,
        ImportMarkdownRequest {
            markdown: r#"---
name: semantic-name
description: New pasted version.
---
"#,
            source_location: None,
        },
    )
    .expect_err("import fails");

    match error {
        ImportError::Collision { name, path } => {
            assert_eq!(name, "semantic-name");
            assert_eq!(path, differently_named_dir);
        }
        error => panic!("unexpected error: {error}"),
    }
}

#[test]
fn markdown_import_rejects_unsafe_skill_names_before_storage() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = DiscoveryRoots {
        canonical_root: temp.path().join("canonical"),
        imports_root: temp.path().join("imports"),
        claude_code_root: temp.path().join("claude"),
        codex_root: temp.path().join("codex"),
    };

    for name in ["../escape", "/absolute", "nested/path"] {
        let markdown = format!(
            r#"---
name: {name}
description: Should not land on disk.
---
"#
        );
        let error = import_markdown_skill(
            &roots,
            ImportMarkdownRequest {
                markdown: &markdown,
                source_location: None,
            },
        )
        .expect_err("import fails");

        match error {
            ImportError::Validation(error) => assert_eq!(error.field, "name"),
            error => panic!("unexpected error: {error}"),
        }
    }

    assert!(
        !roots.imports_root.exists(),
        "unsafe names should not create storage"
    );
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
