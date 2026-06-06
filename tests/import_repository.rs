use std::fs;
use std::path::{Path, PathBuf};

use skill_importer::{
    AgentEnablement, DiscoveryRoots, ImportError, ImportRepositoryRequest, RepositoryImportResult,
    SkillRepositoryCheckout, SkillRepositoryFetchError, SkillRepositoryProvider, SkillSource,
    discover_skills, import_repository_skill,
};

#[test]
fn repository_with_multiple_valid_skills_returns_selection_without_importing() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = roots(temp.path());
    let repository = temp.path().join("repo");
    write_skill(&repository, "repo-alpha", "First repository skill.");
    write_skill(&repository, "repo-beta", "Second repository skill.");
    let provider = StaticRepositoryProvider {
        repository_path: repository,
    };

    let result = import_repository_skill(
        &roots,
        ImportRepositoryRequest {
            repository: "https://example.test/skills.git",
            selected_skill_path: None,
        },
        &provider,
    )
    .expect("repository scan succeeds");

    match result {
        RepositoryImportResult::Selection(selection) => {
            assert_eq!(selection.repository, "https://example.test/skills.git");
            assert_eq!(selection.skills.len(), 2);
            assert_eq!(selection.skills[0].name, "repo-alpha");
            assert_eq!(
                selection.skills[0].description.as_deref(),
                Some("First repository skill.")
            );
            assert_eq!(selection.skills[1].name, "repo-beta");
        }
        RepositoryImportResult::Imported(import) => {
            panic!("expected selection, imported {}", import.skill_name)
        }
    }
    assert!(
        !roots.imports_root.exists(),
        "multi-skill repository selection should not create storage"
    );
}

#[test]
fn repository_with_one_valid_skill_imports_directly() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = roots(temp.path());
    let repository = temp.path().join("repo");
    write_skill(&repository, "solo-repo-skill", "Only repository skill.");
    let provider = StaticRepositoryProvider {
        repository_path: repository,
    };

    let result = import_repository_skill(
        &roots,
        ImportRepositoryRequest {
            repository: "https://example.test/solo.git",
            selected_skill_path: None,
        },
        &provider,
    )
    .expect("repository import succeeds");

    let import = match result {
        RepositoryImportResult::Imported(import) => import,
        RepositoryImportResult::Selection(selection) => {
            panic!("expected import, got {} choices", selection.skills.len())
        }
    };
    assert_eq!(import.skill_name, "solo-repo-skill");
    assert_eq!(
        import.manifest.source_type,
        skill_importer::ImportSourceType::Repository
    );
    assert_eq!(
        import.manifest.source_location.as_deref(),
        Some("https://example.test/solo.git#solo-repo-skill")
    );
    assert!(
        roots
            .imports_root
            .join("solo-repo-skill")
            .join("SKILL.md")
            .exists()
    );

    let inventory = discover_skills(&roots).expect("discovery succeeds");
    assert_eq!(inventory.skills.len(), 1);
    assert_eq!(inventory.skills[0].name, "solo-repo-skill");
    assert_eq!(inventory.skills[0].source, SkillSource::Imported);
    assert_eq!(inventory.skills[0].enablement, AgentEnablement::Neither);
}

#[test]
fn repository_root_skill_imports_directly_with_supporting_files() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = roots(temp.path());
    let repository = temp.path().join("repo");
    fs::create_dir_all(repository.join("scripts")).expect("support dir");
    fs::write(
        repository.join("SKILL.md"),
        r#"---
name: root-repo-skill
description: Root repository skill.
---

# Root Repository Skill
"#,
    )
    .expect("root skill");
    fs::write(repository.join("scripts").join("run.sh"), "echo root\n").expect("support file");
    let provider = StaticRepositoryProvider {
        repository_path: repository,
    };

    let result = import_repository_skill(
        &roots,
        ImportRepositoryRequest {
            repository: "https://example.test/root.git",
            selected_skill_path: None,
        },
        &provider,
    )
    .expect("repository import succeeds");

    let import = match result {
        RepositoryImportResult::Imported(import) => import,
        RepositoryImportResult::Selection(selection) => {
            panic!("expected import, got {} choices", selection.skills.len())
        }
    };
    assert_eq!(import.skill_name, "root-repo-skill");
    assert_eq!(
        import.manifest.source_location.as_deref(),
        Some("https://example.test/root.git#.")
    );
    assert_eq!(
        fs::read_to_string(
            roots
                .imports_root
                .join("root-repo-skill")
                .join("scripts")
                .join("run.sh")
        )
        .expect("stored root support file"),
        "echo root\n"
    );
}

#[test]
fn selected_repository_skill_import_preserves_supporting_files() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = roots(temp.path());
    let repository = temp.path().join("repo");
    write_skill(&repository, "repo-alpha", "First repository skill.");
    let selected = write_skill(&repository, "repo-beta", "Second repository skill.");
    fs::create_dir_all(selected.join("references")).expect("support dir");
    fs::write(
        selected.join("references").join("notes.md"),
        "repository support\n",
    )
    .expect("support file");
    let provider = StaticRepositoryProvider {
        repository_path: repository,
    };

    let result = import_repository_skill(
        &roots,
        ImportRepositoryRequest {
            repository: "https://example.test/many.git",
            selected_skill_path: Some("repo-beta"),
        },
        &provider,
    )
    .expect("selected repository import succeeds");

    let import = match result {
        RepositoryImportResult::Imported(import) => import,
        RepositoryImportResult::Selection(selection) => {
            panic!("expected import, got {} choices", selection.skills.len())
        }
    };
    assert_eq!(import.skill_name, "repo-beta");
    assert!(!roots.imports_root.join("repo-alpha").exists());
    assert_eq!(
        fs::read_to_string(
            roots
                .imports_root
                .join("repo-beta")
                .join("references")
                .join("notes.md")
        )
        .expect("stored support file"),
        "repository support\n"
    );
}

#[test]
fn selected_repository_skill_uses_relative_path_when_names_duplicate() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = roots(temp.path());
    let repository = temp.path().join("repo");
    write_skill_with_frontmatter_name(
        &repository.join("first"),
        "duplicate-name",
        "First duplicate skill.",
    );
    let selected = write_skill_with_frontmatter_name(
        &repository.join("second"),
        "duplicate-name",
        "Second duplicate skill.",
    );
    fs::write(selected.join("chosen.txt"), "selected\n").expect("selected support file");
    let provider = StaticRepositoryProvider {
        repository_path: repository,
    };

    let result = import_repository_skill(
        &roots,
        ImportRepositoryRequest {
            repository: "https://example.test/duplicates.git",
            selected_skill_path: Some("./second"),
        },
        &provider,
    )
    .expect("selected duplicate import succeeds");

    let import = match result {
        RepositoryImportResult::Imported(import) => import,
        RepositoryImportResult::Selection(selection) => {
            panic!("expected import, got {} choices", selection.skills.len())
        }
    };
    assert_eq!(import.skill_name, "duplicate-name");
    assert_eq!(
        fs::read_to_string(roots.imports_root.join("duplicate-name").join("chosen.txt"))
            .expect("stored selected file"),
        "selected\n"
    );
}

#[test]
fn selected_repository_skill_path_that_does_not_exist_returns_clear_error() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = roots(temp.path());
    let repository = temp.path().join("repo");
    write_skill(&repository, "repo-alpha", "First repository skill.");
    write_skill(&repository, "repo-beta", "Second repository skill.");
    let provider = StaticRepositoryProvider {
        repository_path: repository.clone(),
    };

    let error = import_repository_skill(
        &roots,
        ImportRepositoryRequest {
            repository: "https://example.test/many.git",
            selected_skill_path: Some("missing-skill"),
        },
        &provider,
    )
    .expect_err("selected repository import fails");

    match error {
        ImportError::InvalidSource { path, message } => {
            assert_eq!(path, repository);
            assert!(
                message.contains("does not match any skill"),
                "message should explain the missing selection: {message}"
            );
        }
        error => panic!("unexpected error: {error}"),
    }
    assert!(
        !roots.imports_root.exists(),
        "missing selection should not create import storage"
    );
}

#[test]
fn repository_provider_fetch_failure_reports_repository_without_partial_storage() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = roots(temp.path());

    let error = import_repository_skill(
        &roots,
        ImportRepositoryRequest {
            repository: "https://example.test/unavailable.git",
            selected_skill_path: None,
        },
        &FailingRepositoryProvider,
    )
    .expect_err("repository import fails");

    match error {
        ImportError::RepositoryFetch {
            repository,
            message,
        } => {
            assert_eq!(repository, "https://example.test/unavailable.git");
            assert!(
                message.contains("clone failed"),
                "message should explain the fetch failure: {message}"
            );
        }
        error => panic!("unexpected error: {error}"),
    }
    assert!(
        !roots.imports_root.exists(),
        "provider fetch failure should not create import storage"
    );
}

#[test]
fn repository_selection_json_uses_stable_snake_case_kind() {
    let selection = RepositoryImportResult::Selection(skill_importer::RepositorySkillSelection {
        repository: "https://example.test/skills.git".to_string(),
        skills: vec![skill_importer::RepositorySkillCandidate {
            name: "repo-alpha".to_string(),
            description: Some("First repository skill.".to_string()),
            relative_path: "skills/repo-alpha".to_string(),
        }],
    });

    let json = serde_json::to_value(selection).expect("selection json");

    assert_eq!(json["kind"], "selection");
    assert_eq!(json["repository"], "https://example.test/skills.git");
    assert_eq!(json["skills"][0]["name"], "repo-alpha");
    assert_eq!(json["skills"][0]["description"], "First repository skill.");
    assert_eq!(json["skills"][0]["relative_path"], "skills/repo-alpha");
}

#[test]
fn invalid_root_skill_does_not_fall_through_to_nested_support_skill() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = roots(temp.path());
    let repository = temp.path().join("repo");
    fs::create_dir_all(repository.join("examples").join("nested")).expect("nested dir");
    fs::write(
        repository.join("SKILL.md"),
        r#"---
name: broken-root
---
"#,
    )
    .expect("invalid root skill");
    fs::write(
        repository.join("examples").join("nested").join("SKILL.md"),
        r#"---
name: nested-example
description: This should not be imported.
---
"#,
    )
    .expect("nested example skill");
    let provider = StaticRepositoryProvider {
        repository_path: repository,
    };

    let error = import_repository_skill(
        &roots,
        ImportRepositoryRequest {
            repository: "https://example.test/broken-root.git",
            selected_skill_path: None,
        },
        &provider,
    )
    .expect_err("repository import fails");

    match error {
        ImportError::Validation(error) => {
            assert_eq!(error.field, "description");
        }
        error => panic!("unexpected error: {error}"),
    }
    assert!(
        !roots.imports_root.exists(),
        "invalid root skill should not create import storage"
    );
}

#[test]
fn repository_with_no_valid_skills_returns_clear_error_without_partial_storage() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = roots(temp.path());
    let repository = temp.path().join("repo");
    fs::create_dir_all(repository.join("notes")).expect("repo dir");
    fs::write(repository.join("notes").join("README.md"), "not a skill\n").expect("repo file");
    let provider = StaticRepositoryProvider {
        repository_path: repository.clone(),
    };

    let error = import_repository_skill(
        &roots,
        ImportRepositoryRequest {
            repository: "https://example.test/empty.git",
            selected_skill_path: None,
        },
        &provider,
    )
    .expect_err("repository import fails");

    match error {
        ImportError::InvalidSource { path, message } => {
            assert_eq!(path, repository);
            assert!(
                message.contains("no valid skills"),
                "message should explain the empty repository: {message}"
            );
        }
        error => panic!("unexpected error: {error}"),
    }
    assert!(
        !roots.imports_root.exists(),
        "empty repository import should not create storage"
    );
}

struct StaticRepositoryProvider {
    repository_path: PathBuf,
}

struct StaticRepositoryCheckout {
    repository_path: PathBuf,
}

impl SkillRepositoryCheckout for StaticRepositoryCheckout {
    fn path(&self) -> &Path {
        &self.repository_path
    }
}

impl SkillRepositoryProvider for StaticRepositoryProvider {
    type Checkout = StaticRepositoryCheckout;

    fn fetch_repository(
        &self,
        _repository: &str,
    ) -> Result<Self::Checkout, SkillRepositoryFetchError> {
        Ok(StaticRepositoryCheckout {
            repository_path: self.repository_path.clone(),
        })
    }
}

struct FailingRepositoryProvider;

impl SkillRepositoryProvider for FailingRepositoryProvider {
    type Checkout = StaticRepositoryCheckout;

    fn fetch_repository(
        &self,
        _repository: &str,
    ) -> Result<Self::Checkout, SkillRepositoryFetchError> {
        Err(SkillRepositoryFetchError {
            message: "clone failed".to_string(),
        })
    }
}

fn roots(base: &Path) -> DiscoveryRoots {
    DiscoveryRoots {
        canonical_root: base.join("canonical"),
        imports_root: base.join("imports"),
        claude_code_root: base.join("claude"),
        codex_root: base.join("codex"),
    }
}

fn write_skill(root: &Path, name: &str, description: &str) -> PathBuf {
    let skill_dir = root.join(name);
    write_skill_with_frontmatter_name(&skill_dir, name, description)
}

fn write_skill_with_frontmatter_name(skill_dir: &Path, name: &str, description: &str) -> PathBuf {
    fs::create_dir_all(skill_dir).expect("skill dir");
    fs::write(
        skill_dir.join("SKILL.md"),
        format!(
            r#"---
name: {name}
description: {description}
---

# {name}
"#
        ),
    )
    .expect("skill file");
    skill_dir.to_path_buf()
}
