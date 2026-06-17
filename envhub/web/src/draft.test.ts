import { describe, expect, test } from "bun:test"
import { emptyDraft, payloadFromDraft } from "./draft"

describe("payloadFromDraft", () => {
  test("converts editor text into EnvHub payload", () => {
    const dockerfileKey = "dockerfile"
    const draft = {
      ...emptyDraft(),
      name: "demo",
      version: "1.0.0",
      tags: "linux, python",
      artifactsJson: '[{"type":"image","content":"repo/demo:1.0.0"}]',
    }

    const payload = payloadFromDraft(draft)

    expect(payload.name).toBe("demo")
    expect(payload.tags).toEqual(["linux", "python"])
    expect(payload.artifacts[0]?.content).toBe("repo/demo:1.0.0")
    expect(payload.buildConfig[dockerfileKey]).toBe("./Dockerfile")
  })
})
