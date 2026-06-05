use std::fs;
use std::os::unix::fs as unix_fs;

use skill_importer::{
    AgentEnablement, AgentEntryStatus, DiscoveryRoots, SkillSource, discover_skills,
};

#[test]
fn canonical_skill_enabled_for_both_agents_appears_once() {
    let temp = tempfile::tempdir().expect("tempdir");
    let canonical_root = temp.path().join("canonical");
    let imports_root = temp.path().join("imports");
    let claude_root = temp.path().join("claude");
    let codex_root = temp.path().join("codex");

    fs::create_dir_all(&canonical_root).expect("canonical root");
    fs::create_dir_all(&claude_root).expect("claude root");
    fs::create_dir_all(&codex_root).expect("codex root");

    let skill_dir = canonical_root.join("checkout-helper");
    fs::create_dir_all(&skill_dir).expect("skill dir");
    fs::write(
        skill_dir.join("SKILL.md"),
        r#"---
name: checkout-helper
description: Helps checkout flows stay tidy.
---

# Checkout Helper
"#,
    )
    .expect("skill file");

    unix_fs::symlink(&skill_dir, claude_root.join("checkout-helper")).expect("claude symlink");
    unix_fs::symlink(&skill_dir, codex_root.join("checkout-helper")).expect("codex symlink");

    let inventory = discover_skills(&DiscoveryRoots {
        canonical_root,
        imports_root,
        claude_code_root: claude_root,
        codex_root,
    })
    .expect("discovery succeeds with a missing imports root");

    assert_eq!(inventory.skills.len(), 1);

    let skill = &inventory.skills[0];
    assert_eq!(skill.name, "checkout-helper");
    assert_eq!(
        skill.description.as_deref(),
        Some("Helps checkout flows stay tidy.")
    );
    assert_eq!(skill.source, SkillSource::Canonical);
    assert_eq!(skill.enablement, AgentEnablement::Both);
    assert_eq!(
        skill.agent_entries.claude_code,
        AgentEntryStatus::CanonicalSymlink
    );
    assert_eq!(
        skill.agent_entries.codex,
        AgentEntryStatus::CanonicalSymlink
    );
}

#[test]
fn canonical_skill_not_linked_to_agent_roots_is_reported_as_disabled() {
    let temp = tempfile::tempdir().expect("tempdir");
    let canonical_root = temp.path().join("canonical");
    let claude_root = temp.path().join("missing-claude");
    let codex_root = temp.path().join("missing-codex");

    fs::create_dir_all(canonical_root.join("solo-skill")).expect("skill dir");
    fs::write(
        canonical_root.join("solo-skill").join("SKILL.md"),
        r#"---
name: solo-skill
description: Stays available without being enabled.
---
"#,
    )
    .expect("skill file");

    let inventory = discover_skills(&DiscoveryRoots {
        canonical_root,
        imports_root: temp.path().join("missing-imports"),
        claude_code_root: claude_root,
        codex_root,
    })
    .expect("missing roots are treated as empty");

    assert_eq!(inventory.skills.len(), 1);
    assert_eq!(inventory.skills[0].name, "solo-skill");
    assert_eq!(inventory.skills[0].enablement, AgentEnablement::Neither);
    assert_eq!(
        inventory.skills[0].agent_entries.claude_code,
        AgentEntryStatus::Missing
    );
    assert_eq!(
        inventory.skills[0].agent_entries.codex,
        AgentEntryStatus::Missing
    );
}

#[test]
fn canonical_skills_can_be_enabled_for_only_one_agent() {
    let temp = tempfile::tempdir().expect("tempdir");
    let canonical_root = temp.path().join("canonical");
    let claude_root = temp.path().join("claude");
    let codex_root = temp.path().join("codex");

    fs::create_dir_all(&canonical_root).expect("canonical root");
    fs::create_dir_all(&claude_root).expect("claude root");
    fs::create_dir_all(&codex_root).expect("codex root");

    let claude_only = write_skill(
        &canonical_root,
        "claude-only",
        "Only Claude Code uses this.",
    );
    let codex_only = write_skill(&canonical_root, "codex-only", "Only Codex uses this.");

    unix_fs::symlink(&claude_only, claude_root.join("claude-only")).expect("claude symlink");
    unix_fs::symlink(&codex_only, codex_root.join("codex-only")).expect("codex symlink");

    let inventory = discover_skills(&DiscoveryRoots {
        canonical_root,
        imports_root: temp.path().join("missing-imports"),
        claude_code_root: claude_root,
        codex_root,
    })
    .expect("discovery succeeds");

    let claude_skill = inventory
        .skills
        .iter()
        .find(|skill| skill.name == "claude-only")
        .expect("claude-only skill");
    assert_eq!(claude_skill.enablement, AgentEnablement::ClaudeCode);
    assert_eq!(
        claude_skill.agent_entries.claude_code,
        AgentEntryStatus::CanonicalSymlink
    );
    assert_eq!(claude_skill.agent_entries.codex, AgentEntryStatus::Missing);

    let codex_skill = inventory
        .skills
        .iter()
        .find(|skill| skill.name == "codex-only")
        .expect("codex-only skill");
    assert_eq!(codex_skill.enablement, AgentEnablement::Codex);
    assert_eq!(
        codex_skill.agent_entries.claude_code,
        AgentEntryStatus::Missing
    );
    assert_eq!(
        codex_skill.agent_entries.codex,
        AgentEntryStatus::CanonicalSymlink
    );
}

#[test]
fn broken_agent_symlinks_are_reported_without_counting_as_enabled() {
    let temp = tempfile::tempdir().expect("tempdir");
    let claude_root = temp.path().join("claude");
    fs::create_dir_all(&claude_root).expect("claude root");
    unix_fs::symlink(
        temp.path().join("missing-target"),
        claude_root.join("missing-skill"),
    )
    .expect("broken symlink");

    let inventory = discover_skills(&DiscoveryRoots {
        canonical_root: temp.path().join("missing-canonical"),
        imports_root: temp.path().join("missing-imports"),
        claude_code_root: claude_root,
        codex_root: temp.path().join("missing-codex"),
    })
    .expect("discovery succeeds");

    assert_eq!(inventory.skills.len(), 1);
    assert_eq!(inventory.skills[0].name, "missing-skill");
    assert_eq!(inventory.skills[0].enablement, AgentEnablement::Neither);
    assert_eq!(
        inventory.skills[0].agent_entries.claude_code,
        AgentEntryStatus::BrokenSymlink
    );
}

#[test]
fn regular_files_in_agent_roots_do_not_create_skill_entries() {
    let temp = tempfile::tempdir().expect("tempdir");
    let claude_root = temp.path().join("claude");
    fs::create_dir_all(&claude_root).expect("claude root");
    fs::write(claude_root.join("README.md"), "notes, not a skill").expect("regular file");

    let inventory = discover_skills(&DiscoveryRoots {
        canonical_root: temp.path().join("missing-canonical"),
        imports_root: temp.path().join("missing-imports"),
        claude_code_root: claude_root,
        codex_root: temp.path().join("missing-codex"),
    })
    .expect("discovery succeeds");

    assert!(inventory.skills.is_empty());
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
