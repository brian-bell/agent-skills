#!/usr/bin/env node

import { spawn } from "node:child_process";
import { realpathSync } from "node:fs";

const [cwdArgument, extraRootArgument, skillPathArgument] = process.argv.slice(2);
if (!cwdArgument || !extraRootArgument || !skillPathArgument) {
  console.error(
    "usage: check-skills-catalog-codex.mjs <cwd> <extra-skill-root> <skill-path>",
  );
  process.exit(2);
}

const cwd = realpathSync(cwdArgument);
const extraRoot = realpathSync(extraRootArgument);
const skillPath = realpathSync(skillPathArgument);
const expectedAnswer =
  "CATALOG_CODEX_BODY_OK|roles/product-reviewer.md|spawn_agent";

const child = spawn("codex", ["app-server", "--stdio"], {
  stdio: ["pipe", "pipe", "pipe"],
});
let stdoutBuffer = "";
let stderr = "";
let answer = "";
let completed = false;

const timeout = setTimeout(() => fail("Codex app-server smoke timed out"), 60_000);

function send(message) {
  child.stdin.write(`${JSON.stringify(message)}\n`);
}

function fail(message) {
  clearTimeout(timeout);
  child.kill();
  console.error(message);
  if (stderr) {
    console.error(stderr.trimEnd());
  }
  process.exit(1);
}

child.stderr.on("data", (chunk) => {
  stderr += chunk;
});

child.stdout.on("data", (chunk) => {
  stdoutBuffer += chunk;
  for (;;) {
    const newline = stdoutBuffer.indexOf("\n");
    if (newline < 0) {
      break;
    }
    const line = stdoutBuffer.slice(0, newline);
    stdoutBuffer = stdoutBuffer.slice(newline + 1);
    if (!line) {
      continue;
    }
    handleMessage(JSON.parse(line));
  }
});

child.on("exit", (code) => {
  if (!completed && code !== 0) {
    fail(`Codex app-server exited with status ${code}`);
  }
});

function handleMessage(message) {
  if (message.error) {
    fail(`Codex app-server error: ${JSON.stringify(message.error)}`);
  }

  if (message.id === 0) {
    send({ method: "initialized", params: {} });
    send({
      method: "skills/extraRoots/set",
      id: 1,
      params: { extraRoots: [extraRoot] },
    });
    return;
  }

  if (message.id === 1) {
    send({
      method: "skills/list",
      id: 2,
      params: { cwds: [cwd], forceReload: true },
    });
    return;
  }

  if (message.id === 2) {
    const result = message.result?.data?.find((entry) => entry.cwd === cwd);
    const skill = result?.skills?.find((candidate) => candidate.path === skillPath);
    if (!skill) {
      fail(`isolated catalog skill was not discovered at ${skillPath}`);
    }
    if (
      skill.name !== "feature-review" ||
      skill.interface?.displayName !== "Feature Review" ||
      skill.interface?.shortDescription !==
        "Review feature acceptance across five focus areas" ||
      !skill.interface?.defaultPrompt?.includes("$feature-review")
    ) {
      fail(`unexpected feature-review metadata: ${JSON.stringify(skill)}`);
    }
    console.log(
      `CATALOG_CODEX_DISCOVERY_OK|${skill.name}|${skill.interface.displayName}|${skill.path}`,
    );
    send({
      method: "thread/start",
      id: 3,
      params: {
        cwd,
        ephemeral: true,
        approvalPolicy: "never",
        sandbox: "read-only",
      },
    });
    return;
  }

  if (message.id === 3) {
    const thread = message.result?.thread;
    if (!thread?.ephemeral) {
      fail("Codex app-server did not create an ephemeral thread");
    }
    send({
      method: "turn/start",
      id: 4,
      params: {
        threadId: thread.id,
        input: [
          {
            type: "text",
            text: [
              "This is only a packaging smoke test.",
              "Do not perform a review and do not use tools.",
              "From the loaded skill body, return exactly:",
              "CATALOG_CODEX_BODY_OK|<product role relative path>|<native spawn function>.",
              "Do not add other text.",
            ].join(" "),
          },
          { type: "skill", name: "feature-review", path: skillPath },
        ],
      },
    });
    return;
  }

  if (
    message.method === "item/completed" &&
    message.params?.item?.type === "agentMessage"
  ) {
    answer = message.params.item.text.trim();
    return;
  }

  if (message.method === "turn/completed") {
    if (message.params?.turn?.status !== "completed") {
      fail(`Codex turn ended with status ${message.params?.turn?.status}`);
    }
    if (answer !== expectedAnswer) {
      fail(`unexpected Codex skill-body marker: ${JSON.stringify(answer)}`);
    }
    console.log(answer);
    completed = true;
    clearTimeout(timeout);
    child.stdin.end();
  }
}

send({
  method: "initialize",
  id: 0,
  params: {
    clientInfo: {
      name: "agent_skills_catalog_smoke",
      title: "Agent Skills Catalog Smoke",
      version: "1.0.0",
    },
    capabilities: { experimentalApi: true },
  },
});
