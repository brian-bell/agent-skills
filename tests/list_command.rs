use std::fs;
use std::io::Write;
use std::os::unix::fs as unix_fs;
use std::process::{Command, Stdio};

use serde_json::Value;

#[test]
fn list_command_outputs_discovered_inventory_as_json() {
    let temp = tempfile::tempdir().expect("tempdir");
    let canonical_root = temp.path().join("canonical");
    let imports_root = temp.path().join("imports");
    let claude_root = temp.path().join("claude");
    let codex_root = temp.path().join("codex");
    let external_root = temp.path().join("external");

    fs::create_dir_all(&claude_root).expect("claude root");
    fs::create_dir_all(&codex_root).expect("codex root");

    let canonical = write_skill(
        &canonical_root,
        "checkout-helper",
        "Helps checkout flows stay tidy.",
    );
    unix_fs::symlink(&canonical, claude_root.join("checkout-helper")).expect("claude symlink");

    write_skill(&imports_root, "draft-helper", "Imported but not enabled.");
    let imported_enabled = write_skill(
        &imports_root,
        "imported-enabled",
        "Imported and enabled for Codex.",
    );
    unix_fs::symlink(&imported_enabled, codex_root.join("imported-enabled"))
        .expect("imported symlink");

    let external = write_skill(
        &external_root,
        "external-helper",
        "Lives outside managed roots.",
    );
    unix_fs::symlink(&external, codex_root.join("external-helper")).expect("external symlink");

    unix_fs::symlink(
        temp.path().join("missing-target"),
        claude_root.join("broken-helper"),
    )
    .expect("broken symlink");

    write_skill(
        &claude_root,
        "agent-directory",
        "A real directory in an agent root.",
    );

    let output = Command::new(std::env::var("CARGO_BIN_EXE_skill-importer").expect("binary path"))
        .args([
            "list",
            "--json",
            "--canonical-root",
            canonical_root.to_str().expect("canonical root path"),
            "--imports-root",
            imports_root.to_str().expect("imports root path"),
            "--claude-code-root",
            claude_root.to_str().expect("claude root path"),
            "--codex-root",
            codex_root.to_str().expect("codex root path"),
        ])
        .output()
        .expect("run list command");

    assert!(
        output.status.success(),
        "command failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );

    let json: Value = serde_json::from_slice(&output.stdout).expect("valid json output");
    let skills = json["skills"].as_array().expect("skills array");
    assert_eq!(skills.len(), 6, "skills: {skills:?}");

    let checkout_helper = find_skill(skills, "checkout-helper");
    assert_eq!(
        checkout_helper["description"],
        "Helps checkout flows stay tidy."
    );
    assert_eq!(checkout_helper["source"], "canonical");
    assert_eq!(checkout_helper["enablement"]["claude_code"], true);
    assert_eq!(checkout_helper["enablement"]["codex"], false);
    assert_eq!(
        checkout_helper["agent_entries"]["claude_code"],
        "canonical_symlink"
    );
    assert_eq!(checkout_helper["agent_entries"]["codex"], "missing");

    let draft_helper = find_skill(skills, "draft-helper");
    assert_eq!(draft_helper["source"], "imported");
    assert_eq!(draft_helper["enablement"]["claude_code"], false);
    assert_eq!(draft_helper["enablement"]["codex"], false);

    let imported_enabled = find_skill(skills, "imported-enabled");
    assert_eq!(imported_enabled["source"], "imported");
    assert_eq!(
        imported_enabled["agent_entries"]["codex"],
        "imported_symlink"
    );
    assert_eq!(imported_enabled["enablement"]["codex"], true);

    let external_helper = find_skill(skills, "external-helper");
    assert_eq!(
        external_helper["description"],
        "Lives outside managed roots."
    );
    assert_eq!(external_helper["source"], "agent_only");
    assert_eq!(
        external_helper["agent_entries"]["codex"],
        "external_symlink"
    );

    let broken_helper = find_skill(skills, "broken-helper");
    assert_eq!(broken_helper["source"], "agent_only");
    assert_eq!(
        broken_helper["agent_entries"]["claude_code"],
        "broken_symlink"
    );
    assert_eq!(broken_helper["enablement"]["claude_code"], false);

    let agent_directory = find_skill(skills, "agent-directory");
    assert_eq!(agent_directory["source"], "agent_only");
    assert_eq!(
        agent_directory["agent_entries"]["claude_code"],
        "skill_directory"
    );
    assert_eq!(agent_directory["enablement"]["claude_code"], true);
}

#[test]
fn list_command_handles_missing_roots_as_empty_inventory() {
    let temp = tempfile::tempdir().expect("tempdir");
    let canonical_root = temp.path().join("missing-canonical");
    let imports_root = temp.path().join("missing-imports");
    let claude_root = temp.path().join("missing-claude");
    let codex_root = temp.path().join("missing-codex");

    let output = Command::new(std::env::var("CARGO_BIN_EXE_skill-importer").expect("binary path"))
        .args([
            "list",
            "--json",
            "--canonical-root",
            canonical_root.to_str().expect("canonical root path"),
            "--imports-root",
            imports_root.to_str().expect("imports root path"),
            "--claude-code-root",
            claude_root.to_str().expect("claude root path"),
            "--codex-root",
            codex_root.to_str().expect("codex root path"),
        ])
        .output()
        .expect("run list command");

    assert!(
        output.status.success(),
        "command failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );

    let json: Value = serde_json::from_slice(&output.stdout).expect("valid json output");
    assert_eq!(json["skills"].as_array().expect("skills array").len(), 0);
}

#[test]
fn import_markdown_command_reads_stdin_and_outputs_action_json() {
    let temp = tempfile::tempdir().expect("tempdir");
    let canonical_root = temp.path().join("canonical");
    let imports_root = temp.path().join("imports");

    let mut child =
        Command::new(std::env::var("CARGO_BIN_EXE_skill-importer").expect("binary path"))
            .args([
                "import",
                "markdown",
                "--json",
                "--source-location",
                "clipboard",
                "--canonical-root",
                canonical_root.to_str().expect("canonical root path"),
                "--imports-root",
                imports_root.to_str().expect("imports root path"),
            ])
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
            .expect("spawn import command");

    child
        .stdin
        .as_mut()
        .expect("stdin")
        .write_all(
            br#"---
name: command-import
description: Imported through the command.
---

# Command Import
"#,
        )
        .expect("write stdin");

    let output = child.wait_with_output().expect("run import command");
    assert!(
        output.status.success(),
        "command failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );

    let json: Value = serde_json::from_slice(&output.stdout).expect("valid json output");
    assert_eq!(json["skill_name"], "command-import");
    assert_eq!(json["manifest"]["source_type"], "markdown");
    assert_eq!(json["manifest"]["source_location"], "clipboard");
    assert_eq!(json["actions"].as_array().expect("actions").len(), 3);
    assert!(
        imports_root
            .join("command-import")
            .join("SKILL.md")
            .exists()
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

fn find_skill<'skills>(skills: &'skills [Value], name: &str) -> &'skills Value {
    skills
        .iter()
        .find(|skill| skill["name"] == name)
        .expect("skill in json output")
}
