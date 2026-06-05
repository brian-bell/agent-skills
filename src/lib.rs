use std::collections::BTreeMap;
use std::env;
use std::fmt;
use std::fs;
use std::io;
use std::path::{Component, Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};

use serde::Serialize;
use sha2::{Digest, Sha256};
#[cfg(unix)]
use std::os::unix::ffi::OsStrExt;
#[cfg(windows)]
use std::os::windows::ffi::OsStrExt;

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

pub trait SkillUrlFetcher {
    fn fetch_skill_markdown(&self, url: &str) -> Result<String, SkillUrlFetchError>;
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SkillUrlFetchError {
    pub message: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct ImportResult {
    pub skill_name: String,
    pub skill_path: PathBuf,
    pub manifest_path: PathBuf,
    pub manifest: ImportManifest,
    pub actions: Vec<ImportAction>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct ImportManifest {
    pub source_type: ImportSourceType,
    pub source_location: Option<String>,
    pub imported_at: u64,
    pub content_hash: String,
    pub promoted: bool,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum ImportSourceType {
    Markdown,
    LocalPath,
    Url,
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

#[derive(Debug)]
pub enum ImportError {
    Validation(ImportValidationError),
    InvalidSource { path: PathBuf, message: String },
    Fetch { url: String, message: String },
    Collision { name: String, path: PathBuf },
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
    let skill_file_path = match source_kind {
        LocalSkillSourceKind::Directory => source_path.join("SKILL.md"),
        LocalSkillSourceKind::MarkdownFile => source_path.to_path_buf(),
    };
    if !skill_file_path.is_file() {
        return Err(invalid_source_error(
            source_path,
            format!(
                "local skill source must contain {}",
                skill_file_path.display()
            ),
        ));
    }
    if source_kind == LocalSkillSourceKind::Directory {
        refuse_reserved_local_skill_entries(source_path)?;
        refuse_imports_root_inside_source(source_path, &roots.imports_root)?;
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
            | Self::Collision { .. } => None,
        }
    }
}

#[derive(Debug, Clone, Copy)]
enum AgentKind {
    ClaudeCode,
    Codex,
}
