import {
  ArtifactListSchema,
  type EnvMeta,
  type EnvPayload,
  MetadataObjectSchema,
} from "./envSchema"

export type EnvDraft = {
  readonly name: string
  readonly description: string
  readonly version: string
  readonly tags: string
  readonly status: string
  readonly codeUrl: string
  readonly artifactsJson: string
  readonly buildConfigJson: string
  readonly testConfigJson: string
  readonly deployConfigJson: string
}

export function emptyDraft(): EnvDraft {
  return {
    name: "",
    description: "",
    version: "1.0.0",
    tags: "",
    status: "Ready",
    codeUrl: "",
    artifactsJson: "[]",
    buildConfigJson: '{\n  "dockerfile": "./Dockerfile"\n}',
    testConfigJson: '{\n  "script": ""\n}',
    deployConfigJson:
      '{\n  "cpu": "1",\n  "memory": "2Gi",\n  "os": "linux",\n  "ephemeralStorage": "5Gi"\n}',
  }
}

export function draftFromEnv(env: EnvMeta): EnvDraft {
  return {
    name: env.name,
    description: env.description,
    version: env.version,
    tags: env.tags.join(", "),
    status: env.status,
    codeUrl: env.codeUrl,
    artifactsJson: stringifyJson(env.artifacts),
    buildConfigJson: stringifyJson(env.buildConfig),
    testConfigJson: stringifyJson(env.testConfig),
    deployConfigJson: stringifyJson(env.deployConfig),
  }
}

export function payloadFromDraft(draft: EnvDraft): EnvPayload {
  return {
    name: draft.name.trim(),
    description: draft.description.trim(),
    version: draft.version.trim(),
    tags: draft.tags
      .split(",")
      .map((tag) => tag.trim())
      .filter((tag) => tag.length > 0),
    status: draft.status.trim(),
    codeUrl: draft.codeUrl.trim(),
    artifacts: ArtifactListSchema.parse(JSON.parse(draft.artifactsJson)),
    buildConfig: MetadataObjectSchema.parse(JSON.parse(draft.buildConfigJson)),
    testConfig: MetadataObjectSchema.parse(JSON.parse(draft.testConfigJson)),
    deployConfig: MetadataObjectSchema.parse(JSON.parse(draft.deployConfigJson)),
  }
}

function stringifyJson(value: unknown): string {
  return JSON.stringify(value, null, 2)
}
