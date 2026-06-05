use std::collections::BTreeMap;
use std::fs;
use std::io;
use std::path::{Path, PathBuf};

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DiscoveryRoots {
    pub canonical_root: PathBuf,
    pub imports_root: PathBuf,
    pub claude_code_root: PathBuf,
    pub codex_root: PathBuf,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SkillInventory {
    pub skills: Vec<SkillEntry>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SkillEntry {
    pub name: String,
    pub description: Option<String>,
    pub source: SkillSource,
    pub enablement: AgentEnablement,
    pub agent_entries: AgentEntries,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SkillSource {
    Canonical,
    Imported,
    AgentOnly,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct AgentEntries {
    pub claude_code: AgentEntryStatus,
    pub codex: AgentEntryStatus,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum AgentEntryStatus {
    Missing,
    SkillDirectory,
    CanonicalSymlink,
    ImportedSymlink,
    ExternalSymlink,
    BrokenSymlink,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum AgentEnablement {
    Neither,
    ClaudeCode,
    Codex,
    Both,
}

#[derive(Debug, Clone)]
struct SkillDraft {
    name: String,
    description: Option<String>,
    source: SkillSource,
    claude_code_status: AgentEntryStatus,
    codex_status: AgentEntryStatus,
}

#[derive(Debug, Clone)]
struct SkillMetadata {
    name: String,
    description: Option<String>,
}

pub fn discover_skills(roots: &DiscoveryRoots) -> io::Result<SkillInventory> {
    let mut skills = BTreeMap::new();

    discover_skill_collection(&roots.canonical_root, SkillSource::Canonical, &mut skills)?;
    discover_skill_collection(&roots.imports_root, SkillSource::Imported, &mut skills)?;
    discover_agent_root(
        &roots.claude_code_root,
        roots,
        AgentKind::ClaudeCode,
        &mut skills,
    )?;
    discover_agent_root(&roots.codex_root, roots, AgentKind::Codex, &mut skills)?;

    Ok(SkillInventory {
        skills: skills
            .into_values()
            .map(|skill| SkillEntry {
                name: skill.name,
                description: skill.description,
                source: skill.source,
                enablement: AgentEnablement::from_statuses(
                    skill.claude_code_status,
                    skill.codex_status,
                ),
                agent_entries: AgentEntries {
                    claude_code: skill.claude_code_status,
                    codex: skill.codex_status,
                },
            })
            .collect(),
    })
}

fn discover_skill_collection(
    root: &Path,
    source: SkillSource,
    skills: &mut BTreeMap<String, SkillDraft>,
) -> io::Result<()> {
    if !root.exists() {
        return Ok(());
    }

    for entry in fs::read_dir(root)? {
        let entry = entry?;
        let path = entry.path();
        if !entry.file_type()?.is_dir() {
            continue;
        }

        if let Some(metadata) = read_skill_metadata(&path)? {
            merge_skill(skills, metadata, source);
        }
    }

    Ok(())
}

fn discover_agent_root(
    root: &Path,
    roots: &DiscoveryRoots,
    agent: AgentKind,
    skills: &mut BTreeMap<String, SkillDraft>,
) -> io::Result<()> {
    if !root.exists() {
        return Ok(());
    }

    for entry in fs::read_dir(root)? {
        let entry = entry?;
        let path = entry.path();
        let status = agent_entry_status(&path, roots)?;
        if status == AgentEntryStatus::Missing {
            continue;
        }

        let metadata = read_skill_metadata(&path)?.unwrap_or_else(|| SkillMetadata {
            name: entry.file_name().to_string_lossy().into_owned(),
            description: None,
        });

        let skill = skills
            .entry(metadata.name.clone())
            .or_insert_with(|| SkillDraft {
                name: metadata.name,
                description: metadata.description,
                source: SkillSource::AgentOnly,
                claude_code_status: AgentEntryStatus::Missing,
                codex_status: AgentEntryStatus::Missing,
            });

        match agent {
            AgentKind::ClaudeCode => skill.claude_code_status = status,
            AgentKind::Codex => skill.codex_status = status,
        }
    }

    Ok(())
}

fn agent_entry_status(path: &Path, roots: &DiscoveryRoots) -> io::Result<AgentEntryStatus> {
    let symlink_metadata = fs::symlink_metadata(path)?;
    if symlink_metadata.file_type().is_symlink() {
        return match symlink_target(path) {
            Ok(target) if target.is_dir() => Ok(classify_symlink_target(&target, roots)),
            Ok(_) => Ok(AgentEntryStatus::Missing),
            Err(error) if error.kind() == io::ErrorKind::NotFound => {
                Ok(AgentEntryStatus::BrokenSymlink)
            }
            Err(error) => Err(error),
        };
    }

    if symlink_metadata.is_dir() {
        return Ok(AgentEntryStatus::SkillDirectory);
    }

    Ok(AgentEntryStatus::Missing)
}

fn symlink_target(path: &Path) -> io::Result<PathBuf> {
    let target = fs::read_link(path)?;
    let absolute_target = if target.is_absolute() {
        target
    } else {
        path.parent().unwrap_or_else(|| Path::new(".")).join(target)
    };

    fs::canonicalize(absolute_target)
}

fn classify_symlink_target(target: &Path, roots: &DiscoveryRoots) -> AgentEntryStatus {
    if path_is_within_existing_root(target, &roots.canonical_root) {
        return AgentEntryStatus::CanonicalSymlink;
    }

    if path_is_within_existing_root(target, &roots.imports_root) {
        return AgentEntryStatus::ImportedSymlink;
    }

    AgentEntryStatus::ExternalSymlink
}

fn path_is_within_existing_root(path: &Path, root: &Path) -> bool {
    root.exists()
        && fs::canonicalize(root)
            .map(|root| path.starts_with(root))
            .unwrap_or(false)
}

fn merge_skill(
    skills: &mut BTreeMap<String, SkillDraft>,
    metadata: SkillMetadata,
    source: SkillSource,
) {
    skills
        .entry(metadata.name.clone())
        .and_modify(|skill| {
            if source_precedence(source) < source_precedence(skill.source) {
                skill.source = source;
            }
            if skill.description.is_none() {
                skill.description = metadata.description.clone();
            }
        })
        .or_insert_with(|| SkillDraft {
            name: metadata.name,
            description: metadata.description,
            source,
            claude_code_status: AgentEntryStatus::Missing,
            codex_status: AgentEntryStatus::Missing,
        });
}

fn read_skill_metadata(skill_dir: &Path) -> io::Result<Option<SkillMetadata>> {
    let skill_file = skill_dir.join("SKILL.md");
    if !skill_file.exists() {
        return Ok(None);
    }

    let contents = fs::read_to_string(skill_file)?;
    Ok(parse_skill_metadata(&contents))
}

fn parse_skill_metadata(contents: &str) -> Option<SkillMetadata> {
    let mut lines = contents.lines();
    if lines.next()? != "---" {
        return None;
    }

    let mut name = None;
    let mut description = None;

    for line in lines {
        if line == "---" {
            break;
        }

        if let Some(value) = line.strip_prefix("name:") {
            name = Some(clean_frontmatter_value(value));
        } else if let Some(value) = line.strip_prefix("description:") {
            description = Some(clean_frontmatter_value(value));
        }
    }

    name.map(|name| SkillMetadata { name, description })
}

fn clean_frontmatter_value(value: &str) -> String {
    value
        .trim()
        .trim_matches('"')
        .trim_matches('\'')
        .to_string()
}

fn source_precedence(source: SkillSource) -> usize {
    match source {
        SkillSource::Canonical => 0,
        SkillSource::Imported => 1,
        SkillSource::AgentOnly => 2,
    }
}

impl AgentEnablement {
    fn from_statuses(claude_code: AgentEntryStatus, codex: AgentEntryStatus) -> Self {
        match (claude_code.is_enabled(), codex.is_enabled()) {
            (false, false) => Self::Neither,
            (true, false) => Self::ClaudeCode,
            (false, true) => Self::Codex,
            (true, true) => Self::Both,
        }
    }
}

impl AgentEntryStatus {
    fn is_enabled(self) -> bool {
        matches!(
            self,
            Self::SkillDirectory
                | Self::CanonicalSymlink
                | Self::ImportedSymlink
                | Self::ExternalSymlink
        )
    }
}

#[derive(Debug, Clone, Copy)]
enum AgentKind {
    ClaudeCode,
    Codex,
}
