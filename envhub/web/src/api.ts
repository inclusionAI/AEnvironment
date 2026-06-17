import ky from "ky"
import { z } from "zod"
import { EnvListSchema, type EnvMeta, EnvMetaSchema, type EnvPayload } from "./envSchema"

const ApiResponseSchema = z.object({
  success: z.boolean(),
  code: z.number(),
  message: z.string(),
  data: z.unknown(),
})

const apiBase = import.meta.env.VITE_ENVHUB_API_BASE ?? "/env"

export class EnvHubApiError extends Error {
  readonly code: number

  constructor(message: string, code: number) {
    super(message)
    this.name = "EnvHubApiError"
    this.code = code
  }
}

export async function listEnvs(): Promise<readonly EnvMeta[]> {
  const raw = await ky.get(apiBase).json()
  const response = ApiResponseSchema.parse(raw)
  ensureSuccess(response)
  return EnvListSchema.parse(response.data)
}

export async function getEnv(name: string, version: string): Promise<EnvMeta> {
  const raw = await ky
    .get(`${apiBase}/${encodeURIComponent(name)}/${encodeURIComponent(version)}`)
    .json()
  const response = ApiResponseSchema.parse(raw)
  ensureSuccess(response)
  return EnvMetaSchema.parse(response.data)
}

export async function createEnv(payload: EnvPayload): Promise<void> {
  const raw = await ky.post(apiBase, { json: payload }).json()
  const response = ApiResponseSchema.parse(raw)
  ensureSuccess(response)
}

export async function updateEnv(payload: EnvPayload): Promise<void> {
  const name = encodeURIComponent(payload.name)
  const version = encodeURIComponent(payload.version)
  const raw = await ky.put(`${apiBase}/${name}/${version}`, { json: payload }).json()
  const response = ApiResponseSchema.parse(raw)
  ensureSuccess(response)
}

function ensureSuccess(response: z.infer<typeof ApiResponseSchema>): void {
  if (!response.success) {
    throw new EnvHubApiError(response.message || "EnvHub request failed", response.code)
  }
}
