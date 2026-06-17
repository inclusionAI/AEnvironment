import { z } from "zod"

const STATUS_NAMES = [
  "Init",
  "Pending",
  "Creating",
  "Created",
  "Testing",
  "Verified",
  "Ready",
  "Released",
  "Failed",
] as const

const MetadataRecordSchema = z.record(z.string(), z.unknown())

export const ArtifactSchema = z.object({
  id: z.string().optional().default(""),
  type: z.string().optional().default(""),
  content: z.string().optional().default(""),
})

const RawEnvSchema = z
  .object({
    id: z.string().optional().default(""),
    name: z.string().min(1),
    description: z.string().optional().default(""),
    version: z.string().min(1),
    tags: z
      .array(z.string())
      .nullish()
      .transform((tags) => tags ?? []),
    code_url: z.string().optional(),
    codeUrl: z.string().optional(),
    status: z.union([z.string(), z.number()]).optional().default("Init"),
    artifacts: z
      .array(ArtifactSchema)
      .nullish()
      .transform((artifacts) => artifacts ?? []),
    build_config: MetadataRecordSchema.nullish(),
    buildConfig: MetadataRecordSchema.nullish(),
    test_config: MetadataRecordSchema.nullish(),
    testConfig: MetadataRecordSchema.nullish(),
    deploy_config: MetadataRecordSchema.nullish(),
    deployConfig: MetadataRecordSchema.nullish(),
    created_at: z.string().optional().default(""),
    createdAt: z.string().optional(),
    updated_at: z.string().optional().default(""),
    updatedAt: z.string().optional(),
  })
  .transform((raw) => ({
    id: raw.id,
    name: raw.name,
    description: raw.description,
    version: raw.version,
    tags: raw.tags,
    codeUrl: raw.codeUrl ?? raw.code_url ?? "",
    status: statusLabel(raw.status),
    artifacts: raw.artifacts,
    buildConfig: raw.buildConfig ?? raw.build_config ?? {},
    testConfig: raw.testConfig ?? raw.test_config ?? {},
    deployConfig: raw.deployConfig ?? raw.deploy_config ?? {},
    createdAt: raw.createdAt ?? raw.created_at,
    updatedAt: raw.updatedAt ?? raw.updated_at,
  }))

export const EnvMetaSchema = RawEnvSchema
export const EnvListSchema = z
  .array(EnvMetaSchema)
  .nullish()
  .transform((envs) => envs ?? [])
export const ArtifactListSchema = z.array(ArtifactSchema)
export const MetadataObjectSchema = MetadataRecordSchema

export type EnvMeta = z.infer<typeof EnvMetaSchema>
export type Artifact = z.infer<typeof ArtifactSchema>

export type EnvPayload = {
  readonly name: string
  readonly description: string
  readonly version: string
  readonly tags: readonly string[]
  readonly status: string
  readonly codeUrl: string
  readonly artifacts: readonly Artifact[]
  readonly buildConfig: Readonly<Record<string, unknown>>
  readonly testConfig: Readonly<Record<string, unknown>>
  readonly deployConfig: Readonly<Record<string, unknown>>
}

function statusLabel(status: string | number): string {
  if (typeof status === "number") {
    return STATUS_NAMES[status] ?? "Init"
  }
  const lower = status.toLowerCase()
  return STATUS_NAMES.find((name) => name.toLowerCase() === lower) ?? status
}
