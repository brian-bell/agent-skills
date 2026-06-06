use std::fs;
use std::os::unix::fs as unix_fs;
use std::path::{Path, PathBuf};
use std::process::Command;

use serde_json::Value;
use skill_importer::{
    DiscoveryRoots, EnableSkillRequest, ImportLocalPathRequest, ImportMarkdownRequest,
    PromoteSkillRequest, SkillActionKind, SkillAgent, SkillOperationError, enable_skill,
    import_local_path_skill, import_markdown_skill, promote_imported_skill,
};

#[test]
fn promotion_copies_imported_skill_without_import_manifest_and_marks_import_promoted() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = roots(temp.path());
    import_markdown(&roots, "draft-helper");

    let result = promote_imported_skill(
        &roots,
        PromoteSkillRequest {
            skill_name: "draft-helper",
        },
    )
    .expect("promote succeeds");

    assert!(
        roots
            .canonical_root
            .join("draft-helper")
            .join("SKILL.md")
            .exists()
    );
    assert!(
        !roots
            .canonical_root
            .join("draft-helper")
            .join("import.json")
            .exists(),
        "managed import metadata should not be copied into canonical skills"
    );
    let manifest = read_manifest(&roots.imports_root.join("draft-helper").join("import.json"));
    assert_eq!(manifest["promoted"], true);
    assert!(
        result
            .actions
            .iter()
            .any(|action| action.action == SkillActionKind::WriteManifest)
    );
}

#[test]
fn promotion_preserves_supporting_files_from_local_imports() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = roots(temp.path());
    let source = temp.path().join("source").join("support-helper");
    fs::create_dir_all(source.join("references")).expect("support dir");
    fs::write(source.join("SKILL.md"), skill_markdown("support-helper")).expect("skill file");
    fs::write(source.join("references").join("notes.md"), "# Notes\n").expect("support file");
    import_local_path_skill(
        &roots,
        ImportLocalPathRequest {
            path: source.as_path(),
        },
    )
    .expect("import local path");

    promote_imported_skill(
        &roots,
        PromoteSkillRequest {
            skill_name: "support-helper",
        },
    )
    .expect("promote succeeds");

    assert_eq!(
        fs::read_to_string(
            roots
                .canonical_root
                .join("support-helper")
                .join("references")
                .join("notes.md")
        )
        .expect("support file"),
        "# Notes\n"
    );
}

#[test]
fn promotion_relinks_enabled_import_symlinks_to_canonical_skill() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = roots(temp.path());
    let import = import_markdown(&roots, "enabled-helper");
    enable_skill(
        &roots,
        EnableSkillRequest {
            skill_name: "enabled-helper",
            agents: &[SkillAgent::ClaudeCode, SkillAgent::Codex],
        },
    )
    .expect("enable both");

    let result = promote_imported_skill(
        &roots,
        PromoteSkillRequest {
            skill_name: "enabled-helper",
        },
    )
    .expect("promote succeeds");

    let canonical =
        fs::canonicalize(roots.canonical_root.join("enabled-helper")).expect("canonical target");
    assert_eq!(
        fs::canonicalize(roots.claude_code_root.join("enabled-helper")).expect("claude target"),
        canonical
    );
    assert_eq!(
        fs::canonicalize(roots.codex_root.join("enabled-helper")).expect("codex target"),
        canonical
    );
    assert!(
        result
            .actions
            .iter()
            .any(|action| action.action == SkillActionKind::RemoveSymlink
                && action.target == Some(import.skill_path.clone()))
    );
    assert!(
        result
            .actions
            .iter()
            .any(|action| action.action == SkillActionKind::CreateSymlink
                && action.target == Some(canonical.clone()))
    );
}

#[test]
fn promotion_refuses_canonical_collision_before_mutating() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = roots(temp.path());
    let import = import_markdown(&roots, "collision-helper");
    enable_skill(
        &roots,
        EnableSkillRequest {
            skill_name: "collision-helper",
            agents: &[SkillAgent::ClaudeCode],
        },
    )
    .expect("enable");
    write_skill(&roots.canonical_root, "collision-helper");

    let error = promote_imported_skill(
        &roots,
        PromoteSkillRequest {
            skill_name: "collision-helper",
        },
    )
    .expect_err("collision fails");

    assert!(matches!(
        error.error,
        SkillOperationError::Collision { name, .. } if name == "collision-helper"
    ));
    let manifest = read_manifest(
        &roots
            .imports_root
            .join("collision-helper")
            .join("import.json"),
    );
    assert_eq!(manifest["promoted"], false);
    assert_eq!(
        fs::canonicalize(roots.claude_code_root.join("collision-helper")).expect("claude target"),
        import.skill_path
    );
}

#[test]
fn promotion_refuses_unsafe_agent_entries_without_mutating() {
    for case in [
        UnsafeEntry::Directory,
        UnsafeEntry::File,
        UnsafeEntry::ExternalSymlink,
        UnsafeEntry::BrokenSymlink,
        UnsafeEntry::WrongManagedSymlink,
    ] {
        let temp = tempfile::tempdir().expect("tempdir");
        let roots = roots(temp.path());
        import_markdown(&roots, "unsafe-helper");
        place_unsafe_entry(&roots, "unsafe-helper", case);

        let error = promote_imported_skill(
            &roots,
            PromoteSkillRequest {
                skill_name: "unsafe-helper",
            },
        )
        .expect_err("unsafe entry fails");

        assert!(matches!(
            error.error,
            SkillOperationError::UnsafeAgentEntry { .. }
        ));
        assert!(!roots.canonical_root.join("unsafe-helper").exists());
        assert_entry_still_exists(&roots.claude_code_root.join("unsafe-helper"), case);
    }
}

#[test]
fn promotion_reports_unknown_unsupported_and_already_promoted_sources() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = roots(temp.path());

    let unknown = promote_imported_skill(
        &roots,
        PromoteSkillRequest {
            skill_name: "missing-helper",
        },
    )
    .expect_err("unknown");
    assert!(matches!(
        unknown.error,
        SkillOperationError::UnknownSkill { name } if name == "missing-helper"
    ));

    write_skill(&roots.canonical_root, "canonical-helper");
    let canonical = promote_imported_skill(
        &roots,
        PromoteSkillRequest {
            skill_name: "canonical-helper",
        },
    )
    .expect_err("canonical unsupported");
    assert!(matches!(
        canonical.error,
        SkillOperationError::UnsupportedSkillSource { name } if name == "canonical-helper"
    ));

    let agent_only = write_skill(&temp.path().join("external"), "agent-helper");
    fs::create_dir_all(&roots.claude_code_root).expect("claude root");
    unix_fs::symlink(agent_only, roots.claude_code_root.join("agent-helper"))
        .expect("agent symlink");
    let agent = promote_imported_skill(
        &roots,
        PromoteSkillRequest {
            skill_name: "agent-helper",
        },
    )
    .expect_err("agent unsupported");
    assert!(matches!(
        agent.error,
        SkillOperationError::UnsupportedSkillSource { name } if name == "agent-helper"
    ));

    import_markdown(&roots, "promoted-helper");
    promote_imported_skill(
        &roots,
        PromoteSkillRequest {
            skill_name: "promoted-helper",
        },
    )
    .expect("first promote");
    let already = promote_imported_skill(
        &roots,
        PromoteSkillRequest {
            skill_name: "promoted-helper",
        },
    )
    .expect_err("already promoted");
    assert!(matches!(
        already.error,
        SkillOperationError::AlreadyPromoted { name } if name == "promoted-helper"
    ));
}

#[test]
fn promotion_leaves_repo_documentation_and_installer_files_untouched() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = roots(temp.path());
    for file in ["CLAUDE.md", "AGENTS.md", "README.md", "install.sh"] {
        fs::write(temp.path().join(file), format!("{file} sentinel\n")).expect("sentinel");
    }
    import_markdown(&roots, "scoped-helper");

    promote_imported_skill(
        &roots,
        PromoteSkillRequest {
            skill_name: "scoped-helper",
        },
    )
    .expect("promote");

    for file in ["CLAUDE.md", "AGENTS.md", "README.md", "install.sh"] {
        assert_eq!(
            fs::read_to_string(temp.path().join(file)).expect("sentinel"),
            format!("{file} sentinel\n")
        );
    }
}

#[test]
fn promote_command_emits_action_json_and_reports_collisions() {
    let temp = tempfile::tempdir().expect("tempdir");
    let roots = roots(temp.path());
    import_markdown(&roots, "command-promote");

    let output = skill_importer_command()
        .args(["promote", "--json", "--skill", "command-promote"])
        .args(root_args(&roots))
        .output()
        .expect("run promote");
    assert!(
        output.status.success(),
        "promote failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    let json: Value = serde_json::from_slice(&output.stdout).expect("promote json");
    assert_eq!(json["skill_name"], "command-promote");
    assert!(
        json["actions"]
            .as_array()
            .expect("actions")
            .iter()
            .any(|action| action["action"] == "copy_file")
    );

    import_markdown(&roots, "command-collision");
    write_skill(&roots.canonical_root, "command-collision");
    let output = skill_importer_command()
        .args(["promote", "--json", "--skill", "command-collision"])
        .args(root_args(&roots))
        .output()
        .expect("run failing promote");
    assert!(!output.status.success());
    let stderr = String::from_utf8_lossy(&output.stderr);
    assert!(
        stderr.contains("failed to promote skill"),
        "stderr: {stderr}"
    );
    assert!(stderr.contains("already exists"), "stderr: {stderr}");
}

#[derive(Debug, Clone, Copy)]
enum UnsafeEntry {
    Directory,
    File,
    ExternalSymlink,
    BrokenSymlink,
    WrongManagedSymlink,
}

fn roots(base: &Path) -> DiscoveryRoots {
    DiscoveryRoots {
        canonical_root: base.join("canonical"),
        imports_root: base.join("imports"),
        claude_code_root: base.join("claude"),
        codex_root: base.join("codex"),
    }
}

fn import_markdown(roots: &DiscoveryRoots, name: &str) -> skill_importer::ImportResult {
    import_markdown_skill(
        roots,
        ImportMarkdownRequest {
            markdown: &skill_markdown(name),
            source_location: None,
        },
    )
    .expect("import markdown")
}

fn skill_markdown(name: &str) -> String {
    format!(
        r#"---
name: {name}
description: Promotion test skill.
---

# Promotion Test
"#
    )
}

fn write_skill(root: &Path, name: &str) -> PathBuf {
    let skill_dir = root.join(name);
    fs::create_dir_all(&skill_dir).expect("skill dir");
    fs::write(skill_dir.join("SKILL.md"), skill_markdown(name)).expect("skill file");
    fs::canonicalize(skill_dir).expect("canonical skill dir")
}

fn read_manifest(path: &Path) -> Value {
    serde_json::from_str(&fs::read_to_string(path).expect("manifest")).expect("manifest json")
}

fn place_unsafe_entry(roots: &DiscoveryRoots, name: &str, case: UnsafeEntry) {
    fs::create_dir_all(&roots.claude_code_root).expect("claude root");
    let path = roots.claude_code_root.join(name);
    match case {
        UnsafeEntry::Directory => {
            write_skill(&roots.claude_code_root, name);
        }
        UnsafeEntry::File => {
            fs::write(path, "mine").expect("regular file");
        }
        UnsafeEntry::ExternalSymlink => {
            let external = write_skill(&roots.canonical_root.join("external-root"), name);
            unix_fs::symlink(external, path).expect("external symlink");
        }
        UnsafeEntry::BrokenSymlink => {
            unix_fs::symlink(roots.claude_code_root.join("missing"), path).expect("broken symlink");
        }
        UnsafeEntry::WrongManagedSymlink => {
            let other = write_skill(&roots.imports_root, "other-helper");
            unix_fs::symlink(other, path).expect("wrong managed symlink");
        }
    }
}

fn assert_entry_still_exists(path: &Path, case: UnsafeEntry) {
    let metadata = fs::symlink_metadata(path).expect("entry still exists");
    match case {
        UnsafeEntry::Directory => assert!(metadata.is_dir()),
        UnsafeEntry::File => assert!(metadata.is_file()),
        UnsafeEntry::ExternalSymlink
        | UnsafeEntry::BrokenSymlink
        | UnsafeEntry::WrongManagedSymlink => assert!(metadata.file_type().is_symlink()),
    }
}

fn skill_importer_command() -> Command {
    Command::new(std::env::var("CARGO_BIN_EXE_skill-importer").expect("binary path"))
}

fn root_args(roots: &DiscoveryRoots) -> Vec<String> {
    vec![
        "--canonical-root".to_string(),
        roots.canonical_root.to_string_lossy().into_owned(),
        "--imports-root".to_string(),
        roots.imports_root.to_string_lossy().into_owned(),
        "--claude-code-root".to_string(),
        roots.claude_code_root.to_string_lossy().into_owned(),
        "--codex-root".to_string(),
        roots.codex_root.to_string_lossy().into_owned(),
    ]
}
