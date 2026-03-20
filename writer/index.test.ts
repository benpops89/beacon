import { expect, test, describe, beforeEach } from "bun:test";
import { BeaconPlugin, getTmuxSession } from "./index";
import { existsSync, readFileSync, rmSync, mkdirSync } from "node:fs";
import { join } from "node:path";
import { homedir } from "node:os";

const BEACON_DIR = join(homedir(), ".local/share/beacon");

async function getTestFilePath() {
  const sessionName = (await getTmuxSession()).replace(/\//g, "_");
  return join(BEACON_DIR, `${sessionName}.json`);
}

describe("BeaconPlugin Writer", () => {
  beforeEach(async () => {
    if (!existsSync(BEACON_DIR)) {
      mkdirSync(BEACON_DIR, { recursive: true });
    }
    const testFile = await getTestFilePath();
    if (existsSync(testFile)) {
      rmSync(testFile);
    }
  });

  test("should track agent from message.updated event", async () => {
    const plugin = await BeaconPlugin({} as any);
    if (!plugin.event) throw new Error("Plugin event hook not found");

    await plugin.event({
      event: {
        type: "message.updated",
        properties: {
          info: {
            agent: "plan"
          }
        }
      } as any
    });

    await plugin.event({
      event: {
        type: "session.status",
        properties: {
          sessionID: "test-id",
          status: { type: "busy" }
        }
      } as any
    });

    const filePath = await getTestFilePath();
    const content = JSON.parse(readFileSync(filePath, "utf8"));
    expect(content.status).toBe("running");
    expect(content.agent).toBe("plan");
  });

  test("should write 'running' status on busy session event with agent", async () => {
    const plugin = await BeaconPlugin({} as any);
    if (!plugin.event) throw new Error("Plugin event hook not found");

    await plugin.event({
      event: {
        type: "message.updated",
        properties: {
          info: {
            agent: "build"
          }
        }
      } as any
    });

    await plugin.event({
      event: {
        type: "session.status",
        properties: {
          sessionID: "test-id",
          status: { type: "busy" }
        }
      } as any
    });

    const filePath = await getTestFilePath();
    const content = JSON.parse(readFileSync(filePath, "utf8"));
    expect(content.status).toBe("running");
    expect(content.agent).toBe("build");
  });

  test("should write 'input_required' status on idle session event", async () => {
    const plugin = await BeaconPlugin({} as any);
    if (!plugin.event) throw new Error("Plugin event hook not found");

    await plugin.event({
      event: {
        type: "session.status",
        properties: {
          sessionID: "test-id",
          status: { type: "idle" }
        }
      } as any
    });

    const filePath = await getTestFilePath();
    expect(existsSync(filePath)).toBe(true);
    const content = JSON.parse(readFileSync(filePath, "utf8"));
    expect(content.status).toBe("input_required");
  });

  test("beacon_finish tool should mark status as 'finished' with agent", async () => {
    const plugin = await BeaconPlugin({} as any);
    if (!plugin.event) throw new Error("Plugin event hook not found");

    await plugin.event({
      event: {
        type: "message.updated",
        properties: {
          info: {
            agent: "plan"
          }
        }
      } as any
    });

    if (!plugin.tool?.beacon_finish) throw new Error("beacon_finish tool not found");
    await plugin.tool.beacon_finish.execute({}, {} as any);

    const filePath = await getTestFilePath();
    const content = JSON.parse(readFileSync(filePath, "utf8"));
    expect(content.status).toBe("finished");
    expect(content.agent).toBe("plan");
  });
});
