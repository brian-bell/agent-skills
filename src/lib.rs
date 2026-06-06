use std::collections::{BTreeMap, VecDeque};
use std::env;
use std::fmt;
use std::fs;
use std::io;
use std::path::{Component, Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
#[cfg(unix)]
use std::os::unix::ffi::OsStrExt;
#[cfg(windows)]
use std::os::windows::ffi::OsStrExt;

pub mod tui;

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

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct JsonInventory {
    pub skills: Vec<JsonSkillEntry>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ImportMarkdownRequest<'markdown> {
    pub markdown: &'markdown str,
    pub source_location: Option<&'markdown str>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ImportLocalPathRequest<'path> {
    pub path: &'path Path,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ImportUrlRequest<'url> {
    pub url: &'url str,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ImportRepositoryRequest<'repository> {
    pub repository: &'repository str,
    pub selected_skill_path: Option<&'repository str>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum SkillAgent {
    ClaudeCode,
    Codex,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct EnableSkillRequest<'request> {
    pub skill_name: &'request str,
    pub agents: &'request [SkillAgent],
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct DisableSkillRequest<'request> {
    pub skill_name: &'request str,
    pub agents: &'request [SkillAgent],
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct PromoteSkillRequest<'request> {
    pub skill_name: &'request str,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct DeleteImportRequest<'request> {
    pub skill_name: &'request str,
}

pub trait SkillUrlFetcher {
    fn fetch_skill_markdown(&self, url: &str) -> Result<String, SkillUrlFetchError>;
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SkillUrlFetchError {
    pub message: String,
}

pub trait SkillRepositoryCheckout {
    fn path(&self) -> &Path;
}

pub trait SkillRepositoryProvider {
    type Checkout: SkillRepositoryCheckout;

    fn fetch_repository(
        &self,
        repository: &str,
    ) -> Result<Self::Checkout, SkillRepositoryFetchError>;
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SkillRepositoryFetchError {
    pub message: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
#[serde(rename_all = "snake_case", tag = "kind")]
pub enum RepositoryImportResult {
    Imported(ImportResult),
    Selection(RepositorySkillSelection),
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct RepositorySkillSelection {
    pub repository: String,
    pub skills: Vec<RepositorySkillCandidate>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct RepositorySkillCandidate {
    pub name: String,
    pub description: Option<String>,
    pub relative_path: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct ImportResult {
    pub skill_name: String,
    pub skill_path: PathBuf,
    pub manifest_path: PathBuf,
    pub manifest: ImportManifest,
    pub actions: Vec<ImportAction>,
}

#[derive(Debug, Clone, PartialEq, Eq, Deserialize, Serialize)]
pub struct ImportManifest {
    pub source_type: ImportSourceType,
    pub source_location: Option<String>,
    pub imported_at: u64,
    pub content_hash: String,
    pub promoted: bool,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Deserialize, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum ImportSourceType {
    Markdown,
    LocalPath,
    Url,
    Repository,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct ImportAction {
    pub action: ImportActionKind,
    pub path: PathBuf,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum ImportActionKind {
    CreateDirectory,
    WriteSkill,
    CopyFile,
    WriteManifest,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct SkillOperationResult {
    pub skill_name: String,
    pub actions: Vec<SkillAction>,
}

#[derive(Debug)]
pub struct SkillOperationFailure {
    pub error: SkillOperationError,
    pub actions: Vec<SkillAction>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct SkillAction {
    pub action: SkillActionKind,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub agent: Option<SkillAgent>,
    pub path: PathBuf,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub target: Option<PathBuf>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub source: Option<PathBuf>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum SkillActionKind {
    CreateDirectory,
    CreateSymlink,
    RemoveSymlink,
    CopyFile,
    WriteManifest,
    RemoveDirectory,
    SkipUnchanged,
}

#[derive(Debug)]
pub enum ImportError {
    Validation(ImportValidationError),
    InvalidSource { path: PathBuf, message: String },
    Fetch { url: String, message: String },
    RepositoryFetch { repository: String, message: String },
    Collision { name: String, path: PathBuf },
    Io(io::Error),
    Serialize(serde_json::Error),
}

#[derive(Debug)]
pub enum SkillOperationError {
    UnknownSkill { name: String },
    UnsupportedSkillSource { name: String },
    UnsupportedSkillEntry { path: PathBuf, reason: String },
    UnsafeAgentEntry { path: PathBuf, reason: String },
    Collision { name: String, path: PathBuf },
    EnabledImport { name: String, path: PathBuf },
    AlreadyPromoted { name: String },
    Io(io::Error),
    Serialize(serde_json::Error),
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ImportValidationError {
    pub field: &'static str,
    pub message: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct JsonSkillEntry {
    pub name: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    pub source: JsonSkillSource,
    pub enablement: JsonAgentEnablement,
    pub agent_entries: JsonAgentEntries,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum JsonSkillSource {
    Canonical,
    Imported,
    AgentOnly,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
pub struct JsonAgentEnablement {
    pub claude_code: bool,
    pub codex: bool,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
pub struct JsonAgentEntries {
    pub claude_code: JsonAgentEntryStatus,
    pub codex: JsonAgentEntryStatus,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum JsonAgentEntryStatus {
    Missing,
    SkillDirectory,
    CanonicalSymlink,
    ImportedSymlink,
    ExternalSymlink,
    BrokenSymlink,
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

#[derive(Debug, Clone)]
struct RawSkillMetadata {
    name: Option<String>,
    description: Option<String>,
}

pub fn import_markdown_skill(
    roots: &DiscoveryRoots,
    request: ImportMarkdownRequest<'_>,
) -> Result<ImportResult, ImportError> {
    import_markdown_content(
        roots,
        request.markdown,
        ImportSourceType::Markdown,
        request.source_location,
    )
}

pub fn import_url_skill(
    roots: &DiscoveryRoots,
    request: ImportUrlRequest<'_>,
    fetcher: &impl SkillUrlFetcher,
) -> Result<ImportResult, ImportError> {
    let markdown =
        fetcher
            .fetch_skill_markdown(request.url)
            .map_err(|error| ImportError::Fetch {
                url: request.url.to_string(),
                message: error.message,
            })?;

    import_markdown_content(roots, &markdown, ImportSourceType::Url, Some(request.url))
}

pub fn import_repository_skill(
    roots: &DiscoveryRoots,
    request: ImportRepositoryRequest<'_>,
    provider: &impl SkillRepositoryProvider,
) -> Result<RepositoryImportResult, ImportError> {
    let checkout = provider
        .fetch_repository(request.repository)
        .map_err(|error| ImportError::RepositoryFetch {
            repository: request.repository.to_string(),
            message: error.message,
        })?;
    let repository_path = checkout.path();
    let candidates = scan_repository_skills(repository_path)?;

    if candidates.is_empty() {
        return Err(invalid_source_error(
            repository_path,
            "repository contains no valid skills",
        ));
    }

    if let Some(selected_skill_path) = request.selected_skill_path {
        let selected_skill_path = normalize_repository_selector(selected_skill_path)?;
        let matches = candidates
            .iter()
            .filter(|candidate| candidate.relative_path == selected_skill_path)
            .collect::<Vec<_>>();
        let [candidate] = matches.as_slice() else {
            return Err(invalid_source_error(
                repository_path,
                format!(
                    "repository skill selection `{}` does not match any skill in this repository",
                    selected_skill_path
                ),
            ));
        };
        let import =
            import_repository_candidate(roots, request.repository, repository_path, candidate)?;
        return Ok(RepositoryImportResult::Imported(import));
    }

    if candidates.len() == 1 {
        let import = import_repository_candidate(
            roots,
            request.repository,
            repository_path,
            &candidates[0],
        )?;
        return Ok(RepositoryImportResult::Imported(import));
    }

    Ok(RepositoryImportResult::Selection(
        RepositorySkillSelection {
            repository: request.repository.to_string(),
            skills: candidates,
        },
    ))
}

pub fn import_local_path_skill(
    roots: &DiscoveryRoots,
    request: ImportLocalPathRequest<'_>,
) -> Result<ImportResult, ImportError> {
    let source_path = request.path;
    let source_metadata = fs::metadata(source_path).map_err(|error| {
        invalid_source_error(
            source_path,
            format!("failed to read local import source: {error}"),
        )
    })?;
    let source_kind = if source_metadata.is_dir() {
        LocalSkillSourceKind::Directory
    } else if source_metadata.is_file() {
        LocalSkillSourceKind::MarkdownFile
    } else {
        return Err(invalid_source_error(
            source_path,
            "local import source must be a skill directory or Markdown file",
        ));
    };
    if source_kind == LocalSkillSourceKind::Directory {
        return import_skill_directory(
            roots,
            source_path,
            ImportSourceType::LocalPath,
            source_path.to_string_lossy().into_owned(),
        );
    }

    let skill_file_path = source_path.to_path_buf();
    if !skill_file_path.is_file() {
        return Err(invalid_source_error(
            source_path,
            format!(
                "local skill source must contain {}",
                skill_file_path.display()
            ),
        ));
    }
    let markdown = fs::read_to_string(&skill_file_path).map_err(ImportError::Io)?;
    let metadata = validate_import_markdown(&markdown)?;
    let manifest = ImportManifest {
        source_type: ImportSourceType::LocalPath,
        source_location: Some(source_path.to_string_lossy().into_owned()),
        imported_at: current_import_time()?,
        content_hash: local_source_content_hash(source_path, source_kind, &markdown)?,
        promoted: false,
    };

    store_import(roots, metadata, manifest, |skill_path| {
        materialize_local_skill(source_path, skill_path, source_kind)
    })
}

fn store_import(
    roots: &DiscoveryRoots,
    metadata: SkillMetadata,
    manifest: ImportManifest,
    materialize: impl FnOnce(&Path) -> Result<Vec<ImportAction>, ImportError>,
) -> Result<ImportResult, ImportError> {
    let imports_root =
        canonicalize_existing_ancestor(&roots.imports_root).map_err(ImportError::Io)?;
    refuse_collection_collision(
        &metadata.name,
        [roots.canonical_root.as_path(), imports_root.as_path()],
    )?;

    let skill_path = imports_root.join(&metadata.name);
    let manifest_path = skill_path.join("import.json");
    fs::create_dir_all(&imports_root).map_err(ImportError::Io)?;
    fs::create_dir(&skill_path).map_err(|error| {
        if error.kind() == io::ErrorKind::AlreadyExists {
            ImportError::Collision {
                name: metadata.name.clone(),
                path: skill_path.clone(),
            }
        } else {
            ImportError::Io(error)
        }
    })?;
    let content_actions = match materialize(&skill_path) {
        Ok(actions) => actions,
        Err(error) => {
            let _ = fs::remove_dir_all(&skill_path);
            return Err(error);
        }
    };
    if let Err(error) = write_import_manifest(&manifest_path, &manifest) {
        let _ = fs::remove_dir_all(&skill_path);
        return Err(error);
    }

    let skill_name = metadata.name;

    Ok(ImportResult {
        skill_name: skill_name.clone(),
        skill_path: skill_path.clone(),
        manifest_path: manifest_path.clone(),
        manifest,
        actions: import_actions(skill_path, content_actions, manifest_path),
    })
}

fn import_markdown_content(
    roots: &DiscoveryRoots,
    markdown: &str,
    source_type: ImportSourceType,
    source_location: Option<&str>,
) -> Result<ImportResult, ImportError> {
    let metadata = validate_import_markdown(markdown)?;
    let manifest = ImportManifest {
        source_type,
        source_location: source_location.map(str::to_string),
        imported_at: current_import_time()?,
        content_hash: content_hash(markdown),
        promoted: false,
    };

    store_import(roots, metadata, manifest, |skill_path| {
        write_skill_file(skill_path, markdown)
    })
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum LocalSkillSourceKind {
    Directory,
    MarkdownFile,
}

pub fn discover_skills(roots: &DiscoveryRoots) -> io::Result<SkillInventory> {
    let mut skills = BTreeMap::new();
    let roots = DiscoveryRoots {
        canonical_root: roots.canonical_root.clone(),
        imports_root: canonicalize_existing_ancestor(&roots.imports_root)?,
        claude_code_root: roots.claude_code_root.clone(),
        codex_root: roots.codex_root.clone(),
    };

    discover_skill_collection(&roots.canonical_root, SkillSource::Canonical, &mut skills)?;
    discover_skill_collection(&roots.imports_root, SkillSource::Imported, &mut skills)?;
    discover_agent_root(
        &roots.claude_code_root,
        &roots,
        AgentKind::ClaudeCode,
        &mut skills,
    )?;
    discover_agent_root(&roots.codex_root, &roots, AgentKind::Codex, &mut skills)?;

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

pub fn enable_skill(
    roots: &DiscoveryRoots,
    request: EnableSkillRequest<'_>,
) -> Result<SkillOperationResult, SkillOperationFailure> {
    let source = resolve_owned_skill_source(roots, request.skill_name)?;
    let mut actions = Vec::new();
    let plans = preflight_agent_mutation_plans(roots, request.agents, &source)?;

    for plan in plans {
        match plan.state {
            AgentMutationState::Missing => {
                create_agent_root_if_missing(plan.agent, &plan.root, &mut actions)?;
                create_symlink(&source.path, &plan.path)
                    .map_err(SkillOperationError::Io)
                    .map_err(|error| operation_failure(error, actions.clone()))?;
                actions.push(SkillAction {
                    action: SkillActionKind::CreateSymlink,
                    agent: Some(plan.agent),
                    path: plan.path,
                    target: Some(source.path.clone()),
                    source: None,
                });
            }
            AgentMutationState::AlreadyCorrect => {
                actions.push(SkillAction {
                    action: SkillActionKind::SkipUnchanged,
                    agent: Some(plan.agent),
                    path: plan.path,
                    target: Some(source.path.clone()),
                    source: None,
                });
            }
        }
    }

    Ok(SkillOperationResult {
        skill_name: source.name,
        actions,
    })
}

pub fn disable_skill(
    roots: &DiscoveryRoots,
    request: DisableSkillRequest<'_>,
) -> Result<SkillOperationResult, SkillOperationFailure> {
    let source = resolve_owned_skill_source(roots, request.skill_name)?;
    let mut actions = Vec::new();
    let plans = preflight_agent_mutation_plans(roots, request.agents, &source)?;

    for plan in plans {
        match plan.state {
            AgentMutationState::Missing => {
                actions.push(SkillAction {
                    action: SkillActionKind::SkipUnchanged,
                    agent: Some(plan.agent),
                    path: plan.path,
                    target: Some(source.path.clone()),
                    source: None,
                });
            }
            AgentMutationState::AlreadyCorrect => {
                fs::remove_file(&plan.path)
                    .map_err(SkillOperationError::Io)
                    .map_err(|error| operation_failure(error, actions.clone()))?;
                actions.push(SkillAction {
                    action: SkillActionKind::RemoveSymlink,
                    agent: Some(plan.agent),
                    path: plan.path,
                    target: Some(source.path.clone()),
                    source: None,
                });
            }
        }
    }

    Ok(SkillOperationResult {
        skill_name: source.name,
        actions,
    })
}

pub fn promote_imported_skill(
    roots: &DiscoveryRoots,
    request: PromoteSkillRequest<'_>,
) -> Result<SkillOperationResult, SkillOperationFailure> {
    let mut plan = preflight_promotion(roots, request.skill_name)?;
    let mut actions = Vec::new();

    create_directory_if_missing(plan.canonical_root.as_path(), None, &mut actions)?;
    fs::create_dir(&plan.canonical_path)
        .map_err(SkillOperationError::Io)
        .map_err(|error| operation_failure(error, actions.clone()))?;
    actions.push(SkillAction {
        action: SkillActionKind::CreateDirectory,
        agent: None,
        path: plan.canonical_path.clone(),
        target: None,
        source: Some(plan.import_path.clone()),
    });
    copy_operation_skill_directory(
        &plan.import_path,
        &plan.canonical_path,
        CopyMetadataPolicy::ExcludeTopLevelImportManifest,
        &mut actions,
    )?;

    plan.manifest.promoted = true;
    write_operation_import_manifest(&plan.manifest_path, &plan.manifest, &mut actions)?;

    for relink in plan.relinks {
        fs::remove_file(&relink.path)
            .map_err(SkillOperationError::Io)
            .map_err(|error| operation_failure(error, actions.clone()))?;
        actions.push(SkillAction {
            action: SkillActionKind::RemoveSymlink,
            agent: Some(relink.agent),
            path: relink.path.clone(),
            target: Some(plan.import_path.clone()),
            source: None,
        });
        create_symlink(&plan.canonical_path, &relink.path)
            .map_err(SkillOperationError::Io)
            .map_err(|error| operation_failure(error, actions.clone()))?;
        actions.push(SkillAction {
            action: SkillActionKind::CreateSymlink,
            agent: Some(relink.agent),
            path: relink.path,
            target: Some(plan.canonical_path.clone()),
            source: None,
        });
    }

    Ok(SkillOperationResult {
        skill_name: plan.skill_name,
        actions,
    })
}

pub fn delete_unpromoted_import(
    roots: &DiscoveryRoots,
    request: DeleteImportRequest<'_>,
) -> Result<SkillOperationResult, SkillOperationFailure> {
    let plan = preflight_delete_import(roots, request.skill_name)?;
    let mut actions = Vec::new();

    fs::remove_dir_all(&plan.import_path)
        .map_err(SkillOperationError::Io)
        .map_err(|error| operation_failure(error, actions.clone()))?;
    actions.push(SkillAction {
        action: SkillActionKind::RemoveDirectory,
        agent: None,
        path: plan.import_path,
        target: None,
        source: None,
    });

    Ok(SkillOperationResult {
        skill_name: plan.skill_name,
        actions,
    })
}

fn validate_import_markdown(contents: &str) -> Result<SkillMetadata, ImportError> {
    let metadata = parse_skill_frontmatter(contents)?;

    let name = required_frontmatter_field("name", metadata.name)?;
    validate_skill_name(&name)?;
    let description = required_frontmatter_field("description", metadata.description)?;

    Ok(SkillMetadata {
        name,
        description: Some(description),
    })
}

fn parse_skill_frontmatter(contents: &str) -> Result<RawSkillMetadata, ImportError> {
    let mut lines = contents.lines();
    if lines.next() != Some("---") {
        return Err(validation_error(
            "frontmatter",
            "missing opening frontmatter delimiter",
        ));
    }

    let mut name = None;
    let mut description = None;
    let mut closed = false;

    for line in lines {
        if line == "---" {
            closed = true;
            break;
        }

        if let Some(value) = line.strip_prefix("name:") {
            name = Some(clean_frontmatter_value(value));
        } else if let Some(value) = line.strip_prefix("description:") {
            description = Some(clean_frontmatter_value(value));
        }
    }

    if !closed {
        return Err(validation_error(
            "frontmatter",
            "missing closing frontmatter delimiter",
        ));
    }

    Ok(RawSkillMetadata { name, description })
}

fn required_frontmatter_field(
    field: &'static str,
    value: Option<String>,
) -> Result<String, ImportError> {
    let Some(value) = value else {
        return Err(validation_error(field, format!("missing `{field}` field")));
    };

    if value.trim().is_empty() {
        return Err(validation_error(
            field,
            format!("`{field}` cannot be empty"),
        ));
    }

    Ok(value)
}

fn validate_skill_name(name: &str) -> Result<(), ImportError> {
    let mut components = Path::new(name).components();
    let Some(component) = components.next() else {
        return Err(validation_error("name", "`name` cannot be empty"));
    };

    if components.next().is_some() || !matches!(component, std::path::Component::Normal(_)) {
        return Err(validation_error(
            "name",
            "`name` must be a single directory-safe path segment",
        ));
    }

    Ok(())
}

fn refuse_collection_collision<'root>(
    name: &str,
    roots: impl IntoIterator<Item = &'root Path>,
) -> Result<(), ImportError> {
    for root in roots {
        let path = root.join(name);
        if path.exists() || fs::symlink_metadata(&path).is_ok() {
            return Err(ImportError::Collision {
                name: name.to_string(),
                path,
            });
        }

        if !root.exists() {
            continue;
        }

        for entry in fs::read_dir(root).map_err(ImportError::Io)? {
            let entry = entry.map_err(ImportError::Io)?;
            let path = entry.path();
            if !collection_entry_is_skill_dir(&path).map_err(ImportError::Io)? {
                continue;
            }

            if let Some(metadata) = read_skill_metadata(&path).map_err(ImportError::Io)?
                && metadata.name == name
            {
                return Err(ImportError::Collision {
                    name: name.to_string(),
                    path,
                });
            }
        }
    }

    Ok(())
}

fn write_skill_file(skill_path: &Path, markdown: &str) -> Result<Vec<ImportAction>, ImportError> {
    let path = skill_path.join("SKILL.md");
    fs::write(&path, markdown).map_err(ImportError::Io)?;
    Ok(vec![ImportAction {
        action: ImportActionKind::WriteSkill,
        path,
    }])
}

fn write_import_manifest(
    manifest_path: &Path,
    manifest: &ImportManifest,
) -> Result<(), ImportError> {
    let manifest_json = serde_json::to_vec_pretty(manifest).map_err(ImportError::Serialize)?;
    fs::write(manifest_path, manifest_json).map_err(ImportError::Io)?;
    Ok(())
}

fn copy_local_skill_directory(
    source_path: &Path,
    destination_path: &Path,
) -> Result<Vec<ImportAction>, ImportError> {
    let mut actions = Vec::new();
    let mut entries = fs::read_dir(source_path)
        .map_err(ImportError::Io)?
        .collect::<Result<Vec<_>, _>>()
        .map_err(ImportError::Io)?;
    entries.sort_by_key(|entry| entry.file_name());

    for entry in entries {
        let source_entry = entry.path();
        let destination_entry = destination_path.join(entry.file_name());
        copy_local_entry(&source_entry, &destination_entry, &mut actions)?;
    }

    Ok(actions)
}

fn copy_local_entry(
    source_path: &Path,
    destination_path: &Path,
    actions: &mut Vec<ImportAction>,
) -> Result<(), ImportError> {
    let metadata = fs::symlink_metadata(source_path).map_err(ImportError::Io)?;
    if metadata.is_dir() {
        fs::create_dir(destination_path).map_err(ImportError::Io)?;
        actions.push(ImportAction {
            action: ImportActionKind::CreateDirectory,
            path: destination_path.to_path_buf(),
        });
        for action in copy_local_skill_directory(source_path, destination_path)? {
            actions.push(action);
        }
        return Ok(());
    }

    if metadata.is_file() {
        fs::copy(source_path, destination_path).map_err(ImportError::Io)?;
        actions.push(ImportAction {
            action: ImportActionKind::CopyFile,
            path: destination_path.to_path_buf(),
        });
        return Ok(());
    }

    Err(invalid_source_error(
        source_path,
        "unsupported local skill entry; only directories and regular files can be imported",
    ))
}

fn materialize_local_skill(
    source_path: &Path,
    destination_path: &Path,
    source_kind: LocalSkillSourceKind,
) -> Result<Vec<ImportAction>, ImportError> {
    match source_kind {
        LocalSkillSourceKind::Directory => {
            copy_local_skill_directory(source_path, destination_path)
        }
        LocalSkillSourceKind::MarkdownFile => {
            let destination = destination_path.join("SKILL.md");
            fs::copy(source_path, &destination).map_err(ImportError::Io)?;
            Ok(vec![ImportAction {
                action: ImportActionKind::WriteSkill,
                path: destination,
            }])
        }
    }
}

fn local_source_content_hash(
    source_path: &Path,
    source_kind: LocalSkillSourceKind,
    markdown: &str,
) -> Result<String, ImportError> {
    match source_kind {
        LocalSkillSourceKind::Directory => directory_content_hash(source_path),
        LocalSkillSourceKind::MarkdownFile => Ok(content_hash(markdown)),
    }
}

fn refuse_imports_root_inside_source(
    source_path: &Path,
    imports_root: &Path,
) -> Result<(), ImportError> {
    let source_path = fs::canonicalize(source_path).map_err(ImportError::Io)?;
    let imports_root = canonicalize_existing_ancestor(imports_root).map_err(ImportError::Io)?;
    if imports_root.starts_with(&source_path) {
        return Err(invalid_source_error(
            &imports_root,
            "imports root cannot be inside the local skill source",
        ));
    }

    Ok(())
}

fn refuse_reserved_local_skill_entries(source_path: &Path) -> Result<(), ImportError> {
    let import_manifest_path = source_path.join("import.json");
    if fs::symlink_metadata(&import_manifest_path).is_ok() {
        return Err(invalid_source_error(
            &import_manifest_path,
            "`import.json` is reserved for managed import metadata",
        ));
    }

    Ok(())
}

fn canonicalize_existing_ancestor(path: &Path) -> io::Result<PathBuf> {
    let path = if path.is_absolute() {
        path.to_path_buf()
    } else {
        env::current_dir()?.join(path)
    };
    let mut resolved = PathBuf::new();
    let mut components = path.components();

    while let Some(component) = components.next() {
        match component {
            Component::CurDir => {}
            Component::ParentDir => {
                resolved.pop();
            }
            Component::Normal(name) => {
                let candidate = resolved.join(name);
                if candidate.exists() {
                    resolved = fs::canonicalize(candidate)?;
                } else {
                    resolved.push(name);
                    append_missing_components(&mut resolved, components);
                    return Ok(resolved);
                }
            }
            _ => resolved.push(component.as_os_str()),
        }
    }

    Ok(resolved)
}

fn append_missing_components<'path>(
    path: &mut PathBuf,
    components: impl Iterator<Item = Component<'path>>,
) {
    for component in components {
        match component {
            Component::CurDir => {}
            Component::ParentDir => {
                path.pop();
            }
            _ => path.push(component.as_os_str()),
        }
    }
}

fn directory_content_hash(root: &Path) -> Result<String, ImportError> {
    let mut hasher = Sha256::new();
    hash_directory(root, root, &mut hasher)?;
    let digest = hasher.finalize();
    Ok(format!("sha256:{digest:x}"))
}

fn hash_directory(root: &Path, directory: &Path, hasher: &mut Sha256) -> Result<(), ImportError> {
    let mut entries = fs::read_dir(directory)
        .map_err(ImportError::Io)?
        .collect::<Result<Vec<_>, _>>()
        .map_err(ImportError::Io)?;
    entries.sort_by_key(|entry| entry.file_name());

    for entry in entries {
        let path = entry.path();
        let metadata = fs::symlink_metadata(&path).map_err(ImportError::Io)?;
        let relative_path = path.strip_prefix(root).map_err(|error| {
            ImportError::Io(io::Error::other(format!(
                "failed to hash local skill path: {error}"
            )))
        })?;
        if metadata.is_dir() {
            hash_path_record(hasher, b"dir", relative_path);
            hash_directory(root, &path, hasher)?;
        } else if metadata.is_file() {
            let contents = fs::read(&path).map_err(ImportError::Io)?;
            hash_file_record(hasher, relative_path, &contents);
        } else {
            return Err(invalid_source_error(
                &path,
                "unsupported local skill entry; only directories and regular files can be imported",
            ));
        }
    }

    Ok(())
}

fn hash_path_record(hasher: &mut Sha256, tag: &[u8], path: &Path) {
    hasher.update((tag.len() as u64).to_be_bytes());
    hasher.update(tag);
    let path = path_bytes(path);
    hasher.update((path.len() as u64).to_be_bytes());
    hasher.update(path);
}

fn hash_file_record(hasher: &mut Sha256, path: &Path, contents: &[u8]) {
    hash_path_record(hasher, b"file", path);
    hasher.update((contents.len() as u64).to_be_bytes());
    hasher.update(contents);
}

#[cfg(unix)]
fn path_bytes(path: &Path) -> Vec<u8> {
    path.as_os_str().as_bytes().to_vec()
}

#[cfg(not(unix))]
#[cfg(windows)]
fn path_bytes(path: &Path) -> Vec<u8> {
    path.as_os_str()
        .encode_wide()
        .flat_map(u16::to_be_bytes)
        .collect()
}

#[cfg(not(any(unix, windows)))]
fn path_bytes(path: &Path) -> Vec<u8> {
    path.as_os_str().to_string_lossy().as_bytes().to_vec()
}

fn import_actions(
    skill_path: PathBuf,
    content_actions: Vec<ImportAction>,
    manifest_path: PathBuf,
) -> Vec<ImportAction> {
    let mut actions = Vec::with_capacity(content_actions.len() + 2);
    actions.push(ImportAction {
        action: ImportActionKind::CreateDirectory,
        path: skill_path,
    });
    actions.extend(content_actions);
    actions.push(ImportAction {
        action: ImportActionKind::WriteManifest,
        path: manifest_path,
    });
    actions
}

fn current_import_time() -> Result<u64, ImportError> {
    let seconds = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_err(|error| {
            ImportError::Io(io::Error::other(format!(
                "system clock before Unix epoch: {error}"
            )))
        })?
        .as_secs();
    Ok(seconds)
}

fn content_hash(contents: &str) -> String {
    let digest = Sha256::digest(contents.as_bytes());
    format!("sha256:{digest:x}")
}

fn validation_error(field: &'static str, message: impl Into<String>) -> ImportError {
    ImportError::Validation(ImportValidationError {
        field,
        message: message.into(),
    })
}

fn invalid_source_error(path: &Path, message: impl Into<String>) -> ImportError {
    ImportError::InvalidSource {
        path: path.to_path_buf(),
        message: message.into(),
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct OwnedSkillSource {
    name: String,
    path: PathBuf,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum AgentMutationState {
    Missing,
    AlreadyCorrect,
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct AgentMutationPlan {
    agent: SkillAgent,
    root: PathBuf,
    path: PathBuf,
    state: AgentMutationState,
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct PromotionPlan {
    skill_name: String,
    import_path: PathBuf,
    canonical_root: PathBuf,
    canonical_path: PathBuf,
    manifest_path: PathBuf,
    manifest: ImportManifest,
    relinks: Vec<AgentRelinkPlan>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct DeleteImportPlan {
    skill_name: String,
    import_path: PathBuf,
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct ImportPreflight {
    import_path: PathBuf,
    canonical_root: PathBuf,
    canonical_path: PathBuf,
    manifest_path: PathBuf,
    manifest: ImportManifest,
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct AgentRelinkPlan {
    agent: SkillAgent,
    path: PathBuf,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum CopyMetadataPolicy {
    IncludeAll,
    ExcludeTopLevelImportManifest,
}

fn resolve_owned_skill_source(
    roots: &DiscoveryRoots,
    skill_name: &str,
) -> Result<OwnedSkillSource, SkillOperationFailure> {
    let inventory = discover_skills(roots)
        .map_err(SkillOperationError::Io)
        .map_err(empty_operation_failure)?;
    let Some(skill) = inventory
        .skills
        .iter()
        .find(|skill| skill.name == skill_name)
    else {
        return Err(empty_operation_failure(SkillOperationError::UnknownSkill {
            name: skill_name.to_string(),
        }));
    };

    let source_path = match skill.source {
        SkillSource::Canonical => roots.canonical_root.join(skill_name),
        SkillSource::Imported => canonicalize_existing_ancestor(&roots.imports_root)
            .map_err(SkillOperationError::Io)
            .map_err(empty_operation_failure)?
            .join(skill_name),
        SkillSource::AgentOnly => {
            return Err(empty_operation_failure(
                SkillOperationError::UnsupportedSkillSource {
                    name: skill_name.to_string(),
                },
            ));
        }
    };

    let source_path = fs::canonicalize(&source_path)
        .map_err(SkillOperationError::Io)
        .map_err(empty_operation_failure)?;

    Ok(OwnedSkillSource {
        name: skill.name.clone(),
        path: source_path,
    })
}

fn preflight_agent_mutation_plans(
    roots: &DiscoveryRoots,
    agents: &[SkillAgent],
    source: &OwnedSkillSource,
) -> Result<Vec<AgentMutationPlan>, SkillOperationFailure> {
    let mut plans = Vec::new();
    for agent in unique_agents(agents) {
        let root = agent_root(roots, agent);
        let path = root.join(&source.name);
        let state = match exact_managed_symlink_state(&path, &source.path) {
            Ok(state) => state,
            Err(error) => return Err(empty_operation_failure(error)),
        };
        plans.push(AgentMutationPlan {
            agent,
            root,
            path,
            state,
        });
    }

    Ok(plans)
}

fn preflight_promotion(
    roots: &DiscoveryRoots,
    skill_name: &str,
) -> Result<PromotionPlan, SkillOperationFailure> {
    let preflight = resolve_import_preflight(roots, skill_name)?;
    ensure_canonical_destination_available(skill_name, &preflight.canonical_path)?;

    let mut relinks = Vec::new();
    for agent in [SkillAgent::ClaudeCode, SkillAgent::Codex] {
        let path = agent_root(roots, agent).join(skill_name);
        match exact_managed_symlink_state(&path, &preflight.import_path) {
            Ok(AgentMutationState::Missing) => {}
            Ok(AgentMutationState::AlreadyCorrect) => {
                relinks.push(AgentRelinkPlan { agent, path });
            }
            Err(error) => return Err(empty_operation_failure(error)),
        }
    }

    Ok(PromotionPlan {
        skill_name: skill_name.to_string(),
        import_path: preflight.import_path,
        canonical_root: preflight.canonical_root,
        canonical_path: preflight.canonical_path,
        manifest_path: preflight.manifest_path,
        manifest: preflight.manifest,
        relinks,
    })
}

fn preflight_delete_import(
    roots: &DiscoveryRoots,
    skill_name: &str,
) -> Result<DeleteImportPlan, SkillOperationFailure> {
    let preflight = resolve_import_preflight(roots, skill_name)?;

    for agent in [SkillAgent::ClaudeCode, SkillAgent::Codex] {
        let path = agent_root(roots, agent).join(skill_name);
        if agent_entry_points_to(&path, &preflight.import_path)
            .map_err(SkillOperationError::Io)
            .map_err(empty_operation_failure)?
        {
            return Err(empty_operation_failure(
                SkillOperationError::EnabledImport {
                    name: skill_name.to_string(),
                    path,
                },
            ));
        }
    }

    Ok(DeleteImportPlan {
        skill_name: skill_name.to_string(),
        import_path: preflight.import_path,
    })
}

fn resolve_import_preflight(
    roots: &DiscoveryRoots,
    skill_name: &str,
) -> Result<ImportPreflight, SkillOperationFailure> {
    if !skill_name_is_path_segment(skill_name) {
        return Err(empty_operation_failure(SkillOperationError::UnknownSkill {
            name: skill_name.to_string(),
        }));
    }

    let imports_root = canonicalize_existing_ancestor(&roots.imports_root)
        .map_err(SkillOperationError::Io)
        .map_err(empty_operation_failure)?;
    let import_path = imports_root.join(skill_name);
    match fs::symlink_metadata(&import_path) {
        Ok(metadata) if metadata.is_dir() || metadata.file_type().is_symlink() => {}
        Ok(_) => {
            return Err(empty_operation_failure(
                SkillOperationError::UnsupportedSkillSource {
                    name: skill_name.to_string(),
                },
            ));
        }
        Err(error) if error.kind() == io::ErrorKind::NotFound => {
            return Err(unsupported_or_unknown_import_error(roots, skill_name));
        }
        Err(error) => {
            return Err(empty_operation_failure(SkillOperationError::Io(error)));
        }
    }

    let import_path = fs::canonicalize(import_path)
        .map_err(SkillOperationError::Io)
        .map_err(empty_operation_failure)?;
    let manifest_path = import_path.join("import.json");
    let manifest = read_import_manifest(&manifest_path)
        .map_err(SkillOperationError::Io)
        .map_err(empty_operation_failure)?;
    if manifest.promoted {
        return Err(empty_operation_failure(
            SkillOperationError::AlreadyPromoted {
                name: skill_name.to_string(),
            },
        ));
    }

    let canonical_root = canonicalize_existing_ancestor(&roots.canonical_root)
        .map_err(SkillOperationError::Io)
        .map_err(empty_operation_failure)?;
    let canonical_path = canonical_root.join(skill_name);

    Ok(ImportPreflight {
        import_path,
        canonical_root,
        canonical_path,
        manifest_path,
        manifest,
    })
}

fn ensure_canonical_destination_available(
    skill_name: &str,
    canonical_path: &Path,
) -> Result<(), SkillOperationFailure> {
    match fs::symlink_metadata(canonical_path) {
        Ok(_) => {
            return Err(empty_operation_failure(SkillOperationError::Collision {
                name: skill_name.to_string(),
                path: canonical_path.to_path_buf(),
            }));
        }
        Err(error) if error.kind() == io::ErrorKind::NotFound => {}
        Err(error) => return Err(empty_operation_failure(SkillOperationError::Io(error))),
    }

    Ok(())
}

fn unsupported_or_unknown_import_error(
    roots: &DiscoveryRoots,
    skill_name: &str,
) -> SkillOperationFailure {
    let inventory = match discover_skills(roots)
        .map_err(SkillOperationError::Io)
        .map_err(empty_operation_failure)
    {
        Ok(inventory) => inventory,
        Err(failure) => return failure,
    };
    if inventory
        .skills
        .iter()
        .any(|skill| skill.name == skill_name)
    {
        return empty_operation_failure(SkillOperationError::UnsupportedSkillSource {
            name: skill_name.to_string(),
        });
    }

    empty_operation_failure(SkillOperationError::UnknownSkill {
        name: skill_name.to_string(),
    })
}

fn unique_agents(agents: &[SkillAgent]) -> Vec<SkillAgent> {
    let mut unique = Vec::new();
    for agent in agents {
        if !unique.contains(agent) {
            unique.push(*agent);
        }
    }
    unique
}

fn exact_managed_symlink_state(
    path: &Path,
    expected_target: &Path,
) -> Result<AgentMutationState, SkillOperationError> {
    let metadata = match fs::symlink_metadata(path) {
        Ok(metadata) => metadata,
        Err(error) if error.kind() == io::ErrorKind::NotFound => {
            return Ok(AgentMutationState::Missing);
        }
        Err(error) => return Err(SkillOperationError::Io(error)),
    };

    if !metadata.file_type().is_symlink() {
        let reason = if metadata.is_dir() {
            "real directory is not managed by skill-importer"
        } else {
            "regular file is not managed by skill-importer"
        };
        return Err(SkillOperationError::UnsafeAgentEntry {
            path: path.to_path_buf(),
            reason: reason.to_string(),
        });
    }

    match symlink_target(path) {
        Ok(target) if target == expected_target => Ok(AgentMutationState::AlreadyCorrect),
        Ok(target) => Err(SkillOperationError::UnsafeAgentEntry {
            path: path.to_path_buf(),
            reason: format!(
                "symlink points to {} instead of {}",
                target.display(),
                expected_target.display()
            ),
        }),
        Err(error) if error.kind() == io::ErrorKind::NotFound => {
            Err(SkillOperationError::UnsafeAgentEntry {
                path: path.to_path_buf(),
                reason: "broken symlink is not managed by this operation".to_string(),
            })
        }
        Err(error) => Err(SkillOperationError::Io(error)),
    }
}

fn agent_entry_points_to(path: &Path, expected_target: &Path) -> io::Result<bool> {
    let metadata = match fs::symlink_metadata(path) {
        Ok(metadata) => metadata,
        Err(error) if error.kind() == io::ErrorKind::NotFound => return Ok(false),
        Err(error) => return Err(error),
    };
    if !metadata.file_type().is_symlink() {
        return Ok(false);
    }

    match symlink_target(path) {
        Ok(target) => Ok(target == expected_target),
        Err(error) if error.kind() == io::ErrorKind::NotFound => Ok(false),
        Err(error) => Err(error),
    }
}

fn create_agent_root_if_missing(
    agent: SkillAgent,
    root: &Path,
    actions: &mut Vec<SkillAction>,
) -> Result<(), SkillOperationFailure> {
    create_directory_if_missing(root, Some(agent), actions)
}

fn create_directory_if_missing(
    path: &Path,
    agent: Option<SkillAgent>,
    actions: &mut Vec<SkillAction>,
) -> Result<(), SkillOperationFailure> {
    if path.exists() {
        return Ok(());
    }

    fs::create_dir_all(path)
        .map_err(SkillOperationError::Io)
        .map_err(|error| operation_failure(error, actions.clone()))?;
    actions.push(SkillAction {
        action: SkillActionKind::CreateDirectory,
        agent,
        path: path.to_path_buf(),
        target: None,
        source: None,
    });
    Ok(())
}

fn agent_root(roots: &DiscoveryRoots, agent: SkillAgent) -> PathBuf {
    match agent {
        SkillAgent::ClaudeCode => roots.claude_code_root.clone(),
        SkillAgent::Codex => roots.codex_root.clone(),
    }
}

fn read_import_manifest(manifest_path: &Path) -> io::Result<ImportManifest> {
    let contents = fs::read(manifest_path)?;
    serde_json::from_slice(&contents)
        .map_err(|error| io::Error::new(io::ErrorKind::InvalidData, error))
}

fn write_operation_import_manifest(
    manifest_path: &Path,
    manifest: &ImportManifest,
    actions: &mut Vec<SkillAction>,
) -> Result<(), SkillOperationFailure> {
    let manifest_json = serde_json::to_vec_pretty(manifest)
        .map_err(SkillOperationError::Serialize)
        .map_err(|error| operation_failure(error, actions.clone()))?;
    fs::write(manifest_path, manifest_json)
        .map_err(SkillOperationError::Io)
        .map_err(|error| operation_failure(error, actions.clone()))?;
    actions.push(SkillAction {
        action: SkillActionKind::WriteManifest,
        agent: None,
        path: manifest_path.to_path_buf(),
        target: None,
        source: None,
    });
    Ok(())
}

fn copy_operation_skill_directory(
    source_path: &Path,
    destination_path: &Path,
    metadata_policy: CopyMetadataPolicy,
    actions: &mut Vec<SkillAction>,
) -> Result<(), SkillOperationFailure> {
    let mut entries = fs::read_dir(source_path)
        .map_err(SkillOperationError::Io)
        .map_err(|error| operation_failure(error, actions.clone()))?
        .collect::<Result<Vec<_>, _>>()
        .map_err(SkillOperationError::Io)
        .map_err(|error| operation_failure(error, actions.clone()))?;
    entries.sort_by_key(|entry| entry.file_name());

    for entry in entries {
        if metadata_policy == CopyMetadataPolicy::ExcludeTopLevelImportManifest
            && entry.file_name() == "import.json"
        {
            continue;
        }
        let source_entry = entry.path();
        let destination_entry = destination_path.join(entry.file_name());
        copy_operation_skill_entry(&source_entry, &destination_entry, actions)?;
    }

    Ok(())
}

fn copy_operation_skill_entry(
    source_path: &Path,
    destination_path: &Path,
    actions: &mut Vec<SkillAction>,
) -> Result<(), SkillOperationFailure> {
    let metadata = fs::symlink_metadata(source_path)
        .map_err(SkillOperationError::Io)
        .map_err(|error| operation_failure(error, actions.clone()))?;
    if metadata.is_dir() {
        fs::create_dir(destination_path)
            .map_err(SkillOperationError::Io)
            .map_err(|error| operation_failure(error, actions.clone()))?;
        actions.push(SkillAction {
            action: SkillActionKind::CreateDirectory,
            agent: None,
            path: destination_path.to_path_buf(),
            target: None,
            source: Some(source_path.to_path_buf()),
        });
        copy_operation_skill_directory(
            source_path,
            destination_path,
            CopyMetadataPolicy::IncludeAll,
            actions,
        )?;
        return Ok(());
    }

    if metadata.is_file() {
        fs::copy(source_path, destination_path)
            .map_err(SkillOperationError::Io)
            .map_err(|error| operation_failure(error, actions.clone()))?;
        actions.push(SkillAction {
            action: SkillActionKind::CopyFile,
            agent: None,
            path: destination_path.to_path_buf(),
            target: None,
            source: Some(source_path.to_path_buf()),
        });
        return Ok(());
    }

    Err(operation_failure(
        SkillOperationError::UnsupportedSkillEntry {
            path: source_path.to_path_buf(),
            reason: "unsupported imported skill entry".to_string(),
        },
        actions.clone(),
    ))
}

fn skill_name_is_path_segment(name: &str) -> bool {
    let mut components = Path::new(name).components();
    matches!(components.next(), Some(Component::Normal(_))) && components.next().is_none()
}

fn empty_operation_failure(error: SkillOperationError) -> SkillOperationFailure {
    operation_failure(error, Vec::new())
}

fn operation_failure(
    error: SkillOperationError,
    actions: Vec<SkillAction>,
) -> SkillOperationFailure {
    SkillOperationFailure { error, actions }
}

#[cfg(unix)]
fn create_symlink(target: &Path, link: &Path) -> io::Result<()> {
    std::os::unix::fs::symlink(target, link)
}

#[cfg(windows)]
fn create_symlink(target: &Path, link: &Path) -> io::Result<()> {
    std::os::windows::fs::symlink_dir(target, link)
}

fn scan_repository_skills(
    repository_path: &Path,
) -> Result<Vec<RepositorySkillCandidate>, ImportError> {
    let mut candidates = Vec::new();
    if let Some(candidate) =
        repository_skill_candidate(repository_path, repository_path, RootSkillPolicy::Strict)?
    {
        candidates.push(candidate);
        return Ok(candidates);
    }
    scan_repository_directory(repository_path, repository_path, &mut candidates)?;
    candidates.sort_by(|left, right| left.relative_path.cmp(&right.relative_path));
    Ok(candidates)
}

fn import_repository_candidate(
    roots: &DiscoveryRoots,
    repository: &str,
    repository_path: &Path,
    candidate: &RepositorySkillCandidate,
) -> Result<ImportResult, ImportError> {
    let source_path = repository_path.join(repository_path_from_selector(&candidate.relative_path));
    let source_location = format!("{repository}#{}", candidate.relative_path);
    import_skill_directory(
        roots,
        &source_path,
        ImportSourceType::Repository,
        source_location,
    )
}

fn import_skill_directory(
    roots: &DiscoveryRoots,
    source_path: &Path,
    source_type: ImportSourceType,
    source_location: String,
) -> Result<ImportResult, ImportError> {
    let skill_file_path = source_path.join("SKILL.md");
    if !skill_file_path.is_file() {
        return Err(invalid_source_error(
            source_path,
            format!("skill source must contain {}", skill_file_path.display()),
        ));
    }
    refuse_reserved_local_skill_entries(source_path)?;
    refuse_imports_root_inside_source(source_path, &roots.imports_root)?;
    let markdown = fs::read_to_string(&skill_file_path).map_err(ImportError::Io)?;
    let metadata = validate_import_markdown(&markdown)?;
    let manifest = ImportManifest {
        source_type,
        source_location: Some(source_location),
        imported_at: current_import_time()?,
        content_hash: directory_content_hash(source_path)?,
        promoted: false,
    };

    store_import(roots, metadata, manifest, |skill_path| {
        materialize_local_skill(source_path, skill_path, LocalSkillSourceKind::Directory)
    })
}

fn scan_repository_directory(
    repository_path: &Path,
    directory_path: &Path,
    candidates: &mut Vec<RepositorySkillCandidate>,
) -> Result<(), ImportError> {
    const MAX_REPOSITORY_SCAN_DEPTH: usize = 8;

    let mut queue = VecDeque::from([(directory_path.to_path_buf(), 0_usize)]);
    while let Some((directory, depth)) = queue.pop_front() {
        let mut entries = fs::read_dir(&directory)
            .map_err(ImportError::Io)?
            .collect::<Result<Vec<_>, _>>()
            .map_err(ImportError::Io)?;
        entries.sort_by_key(|entry| entry.file_name());

        for entry in entries {
            let path = entry.path();
            let metadata = fs::symlink_metadata(&path).map_err(ImportError::Io)?;
            // Do not follow symlinked directories while scanning untrusted repository checkouts.
            if !metadata.is_dir() {
                continue;
            }

            if let Some(candidate) =
                repository_skill_candidate(repository_path, &path, RootSkillPolicy::IgnoreInvalid)?
            {
                candidates.push(candidate);
                continue;
            }

            if depth < MAX_REPOSITORY_SCAN_DEPTH {
                queue.push_back((path, depth + 1));
            }
        }
    }

    Ok(())
}

fn repository_skill_candidate(
    repository_path: &Path,
    directory_path: &Path,
    root_skill_policy: RootSkillPolicy,
) -> Result<Option<RepositorySkillCandidate>, ImportError> {
    let skill_path = directory_path.join("SKILL.md");
    if !skill_path.is_file() {
        return Ok(None);
    }

    let markdown = fs::read_to_string(&skill_path).map_err(ImportError::Io)?;
    let metadata = match validate_import_markdown(&markdown) {
        Ok(metadata) => metadata,
        Err(error) => match root_skill_policy {
            RootSkillPolicy::Strict => return Err(error),
            RootSkillPolicy::IgnoreInvalid => return Ok(None),
        },
    };
    let relative_path = repository_relative_path(repository_path, directory_path)?;

    Ok(Some(RepositorySkillCandidate {
        name: metadata.name,
        description: metadata.description,
        relative_path,
    }))
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum RootSkillPolicy {
    Strict,
    IgnoreInvalid,
}

fn repository_relative_path(
    repository_path: &Path,
    directory_path: &Path,
) -> Result<String, ImportError> {
    let relative_path = directory_path
        .strip_prefix(repository_path)
        .map_err(|error| {
            ImportError::Io(io::Error::other(format!(
                "failed to read repository skill path: {error}"
            )))
        })?;
    if relative_path.as_os_str().is_empty() {
        return Ok(".".to_string());
    }

    let mut parts = Vec::new();
    for component in relative_path.components() {
        let Component::Normal(part) = component else {
            return Err(invalid_source_error(
                directory_path,
                "repository skill paths must be relative directory paths",
            ));
        };
        let part = part.to_str().ok_or_else(|| {
            invalid_source_error(directory_path, "repository skill paths must be valid UTF-8")
        })?;
        parts.push(part.to_string());
    }

    Ok(parts.join("/"))
}

fn normalize_repository_selector(selector: &str) -> Result<String, ImportError> {
    if selector.trim().is_empty() || selector.starts_with('/') || selector.contains('\\') {
        return Err(invalid_source_error(
            Path::new(selector),
            "repository skill selection must be a relative slash-delimited path",
        ));
    }

    let mut parts = Vec::new();
    for part in selector.split('/') {
        match part {
            "" | "." => {}
            ".." => {
                return Err(invalid_source_error(
                    Path::new(selector),
                    "repository skill selection cannot contain parent path segments",
                ));
            }
            part => parts.push(part),
        }
    }

    if parts.is_empty() {
        Ok(".".to_string())
    } else {
        Ok(parts.join("/"))
    }
}

fn repository_path_from_selector(selector: &str) -> PathBuf {
    if selector == "." {
        return PathBuf::new();
    }

    selector.split('/').collect()
}

pub fn inventory_to_json(inventory: &SkillInventory) -> JsonInventory {
    JsonInventory {
        skills: inventory
            .skills
            .iter()
            .map(|skill| JsonSkillEntry {
                name: skill.name.clone(),
                description: skill.description.clone(),
                source: skill.source.into(),
                enablement: JsonAgentEnablement {
                    claude_code: skill.agent_entries.claude_code.is_enabled(),
                    codex: skill.agent_entries.codex.is_enabled(),
                },
                agent_entries: JsonAgentEntries {
                    claude_code: skill.agent_entries.claude_code.into(),
                    codex: skill.agent_entries.codex.into(),
                },
            })
            .collect(),
    }
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
        if !collection_entry_is_skill_dir(&path)? {
            continue;
        }

        if let Some(metadata) = read_skill_metadata(&path)? {
            merge_skill(skills, metadata, source);
        }
    }

    Ok(())
}

fn collection_entry_is_skill_dir(path: &Path) -> io::Result<bool> {
    let metadata = fs::symlink_metadata(path)?;
    if metadata.is_dir() {
        return Ok(true);
    }

    if metadata.file_type().is_symlink() {
        return match fs::metadata(path) {
            Ok(target_metadata) => Ok(target_metadata.is_dir()),
            Err(error) if error.kind() == io::ErrorKind::NotFound => Ok(false),
            Err(error) => Err(error),
        };
    }

    Ok(false)
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
    let value = value.trim();
    if let Some(value) = value
        .strip_prefix('"')
        .and_then(|value| value.strip_suffix('"'))
    {
        return value.to_string();
    }
    if let Some(value) = value
        .strip_prefix('\'')
        .and_then(|value| value.strip_suffix('\''))
    {
        return value.to_string();
    }

    value.to_string()
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

impl From<AgentEntryStatus> for JsonAgentEntryStatus {
    fn from(status: AgentEntryStatus) -> Self {
        match status {
            AgentEntryStatus::Missing => Self::Missing,
            AgentEntryStatus::SkillDirectory => Self::SkillDirectory,
            AgentEntryStatus::CanonicalSymlink => Self::CanonicalSymlink,
            AgentEntryStatus::ImportedSymlink => Self::ImportedSymlink,
            AgentEntryStatus::ExternalSymlink => Self::ExternalSymlink,
            AgentEntryStatus::BrokenSymlink => Self::BrokenSymlink,
        }
    }
}

impl From<SkillSource> for JsonSkillSource {
    fn from(source: SkillSource) -> Self {
        match source {
            SkillSource::Canonical => Self::Canonical,
            SkillSource::Imported => Self::Imported,
            SkillSource::AgentOnly => Self::AgentOnly,
        }
    }
}

impl fmt::Display for ImportError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Validation(error) => write!(formatter, "{}: {}", error.field, error.message),
            Self::InvalidSource { path, message } => {
                write!(
                    formatter,
                    "invalid local import source {}: {message}",
                    path.display()
                )
            }
            Self::Fetch { url, message } => {
                write!(formatter, "failed to fetch skill URL {url}: {message}")
            }
            Self::RepositoryFetch {
                repository,
                message,
            } => {
                write!(
                    formatter,
                    "failed to fetch repository {repository}: {message}"
                )
            }
            Self::Collision { name, path } => write!(
                formatter,
                "skill `{name}` already exists at {}",
                path.display()
            ),
            Self::Io(error) => write!(formatter, "{error}"),
            Self::Serialize(error) => write!(formatter, "{error}"),
        }
    }
}

impl std::error::Error for ImportError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            Self::Io(error) => Some(error),
            Self::Serialize(error) => Some(error),
            Self::Validation(_)
            | Self::InvalidSource { .. }
            | Self::Fetch { .. }
            | Self::RepositoryFetch { .. }
            | Self::Collision { .. } => None,
        }
    }
}

impl fmt::Display for SkillOperationError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::UnknownSkill { name } => write!(formatter, "unknown skill `{name}`"),
            Self::UnsupportedSkillSource { name } => write!(
                formatter,
                "skill `{name}` is agent-only and cannot be managed by skill-importer"
            ),
            Self::UnsupportedSkillEntry { path, reason } => {
                write!(
                    formatter,
                    "unsupported skill entry {}: {reason}",
                    path.display()
                )
            }
            Self::UnsafeAgentEntry { path, reason } => {
                write!(formatter, "unsafe agent entry {}: {reason}", path.display())
            }
            Self::Collision { name, path } => write!(
                formatter,
                "skill `{name}` already exists at {}",
                path.display()
            ),
            Self::EnabledImport { name, path } => write!(
                formatter,
                "import `{name}` is still enabled at {}; disable it first",
                path.display()
            ),
            Self::AlreadyPromoted { name } => {
                write!(formatter, "import `{name}` has already been promoted")
            }
            Self::Io(error) => write!(formatter, "{error}"),
            Self::Serialize(error) => write!(formatter, "{error}"),
        }
    }
}

impl std::error::Error for SkillOperationError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            Self::Io(error) => Some(error),
            Self::Serialize(error) => Some(error),
            Self::UnknownSkill { .. }
            | Self::UnsupportedSkillSource { .. }
            | Self::UnsupportedSkillEntry { .. }
            | Self::UnsafeAgentEntry { .. }
            | Self::Collision { .. }
            | Self::EnabledImport { .. }
            | Self::AlreadyPromoted { .. } => None,
        }
    }
}

#[derive(Debug, Clone, Copy)]
enum AgentKind {
    ClaudeCode,
    Codex,
}
