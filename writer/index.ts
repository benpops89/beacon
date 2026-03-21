import { type Plugin, tool } from "@opencode-ai/plugin";
import { $ } from "bun";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { homedir } from "node:os";

const BEACON_DIR = join(homedir(), ".local/share/beacon");

/**
 * Resolves the current tmux session name or falls back to the directory name.
 */
export async function getTmuxSession(): Promise<string> {
  try {
    const session = await $`tmux display-message -p "#S"`.text();
    const trimmed = session.trim();
    if (trimmed && trimmed !== "") return trimmed;
  } catch {
    // Fallback if not in tmux or tmux fails
  }
  return process.cwd().split("/").pop() || "unknown";
}

/**
 * Writes the session status to ~/.local/share/beacon/[session].json.
 */
async function updateStatus(status: "running" | "input_required" | "finished", agent?: string) {
  const sessionName = (await getTmuxSession().catch(() => "unknown")).replace(/\//g, "_");
  const filePath = join(BEACON_DIR, `${sessionName}.json`);

  mkdirSync(BEACON_DIR, { recursive: true });

  const payload: Record<string, string> = {
    status,
    updated_at: new Date().toISOString(),
    session_name: sessionName,
  };

  if (agent) {
    payload.agent = agent;
  }

  writeFileSync(filePath, JSON.stringify(payload, null, 2));
}

let currentAgent = "unknown";

export const BeaconPlugin: Plugin = async () => {
  return {
    event: async ({ event }) => {
      if (event.type === "message.updated") {
        const info = (event as any).properties?.info;
        if (info?.agent) {
          currentAgent = info.agent;
        }
      }

      if (event.type === "session.status") {
        if (event.properties.status.type === "busy") {
          await updateStatus("running", currentAgent);
        } else if (event.properties.status.type === "idle") {
          await updateStatus("input_required");
        }
      }
      if (event.type === "permission.updated") {
        await updateStatus("input_required");
      }
    },

    tool: {
      beacon_finish: tool({
        description: "Mark the current task as finished in the beacon status tracker.",
        args: {},
        async execute() {
          await updateStatus("finished", currentAgent);
          currentAgent = "unknown";
          return "Task marked as finished.";
        },
      }),
    },
  };
};

export default BeaconPlugin;
