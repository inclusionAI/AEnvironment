import { describe, expect, test } from "bun:test"
import { EnvListSchema, EnvMetaSchema } from "./envSchema"

describe("EnvMetaSchema", () => {
  test("normalizes snake case EnvHub responses", () => {
    const dockerfileKey = "dockerfile"
    const env = EnvMetaSchema.parse({
      id: "demo-1.0.0",
      name: "demo",
      description: "Demo env",
      version: "1.0.0",
      tags: ["linux"],
      status: 6,
      code_url: "oss://demo",
      artifacts: [{ type: "image", content: "repo/demo:1.0.0" }],
      build_config: { dockerfile: "./Dockerfile" },
      test_config: { script: "pytest" },
      deploy_config: { cpu: "1", memory: "2Gi" },
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-02T00:00:00Z",
    })

    expect(env.codeUrl).toBe("oss://demo")
    expect(env.status).toBe("Ready")
    expect(env.buildConfig[dockerfileKey]).toBe("./Dockerfile")
    expect(env.updatedAt).toBe("2026-01-02T00:00:00Z")
  })

  test("accepts camel case editor payloads", () => {
    const dockerfileKey = "dockerfile"
    const env = EnvMetaSchema.parse({
      name: "demo",
      version: "1.0.1",
      status: "Ready",
      codeUrl: "oss://demo-new",
      buildConfig: { dockerfile: "./Containerfile" },
      testConfig: {},
      deployConfig: {},
    })

    expect(env.codeUrl).toBe("oss://demo-new")
    expect(env.buildConfig[dockerfileKey]).toBe("./Containerfile")
  })

  test("accepts null collection fields from sparse EnvHub records", () => {
    const env = EnvMetaSchema.parse({
      name: "sparse",
      version: "1.0.0",
      tags: null,
      artifacts: null,
      build_config: null,
      test_config: null,
      deploy_config: null,
    })

    expect(env.tags).toEqual([])
    expect(env.artifacts).toEqual([])
    expect(env.buildConfig).toEqual({})
  })
})

describe("EnvListSchema", () => {
  test("accepts null empty list responses from older EnvHub servers", () => {
    expect(EnvListSchema.parse(null)).toEqual([])
  })
})
