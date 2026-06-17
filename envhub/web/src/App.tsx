import { useCallback, useEffect, useState } from "react"
import { createEnv, getEnv, listEnvs, updateEnv } from "./api"
import { EnvEditor } from "./components/EnvEditor"
import { EnvList } from "./components/EnvList"
import { EnvSummary } from "./components/EnvSummary"
import { draftFromEnv, type EnvDraft, emptyDraft, payloadFromDraft } from "./draft"
import type { EnvMeta } from "./envSchema"

export function App() {
  const [envs, setEnvs] = useState<readonly EnvMeta[]>([])
  const [selected, setSelected] = useState<EnvMeta | null>(null)
  const [draft, setDraft] = useState<EnvDraft>(emptyDraft())
  const [mode, setMode] = useState<"create" | "edit">("create")
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState("")
  const [error, setError] = useState("")

  const refreshList = useCallback(
    async (selectFirst: boolean) => {
      setLoading(true)
      setError("")
      try {
        const nextEnvs = await listEnvs()
        setEnvs(nextEnvs)
        if (selectFirst && selected === null && nextEnvs.length > 0) {
          const firstEnv = nextEnvs[0]
          if (firstEnv !== undefined) {
            setSelected(firstEnv)
            setDraft(draftFromEnv(firstEnv))
            setMode("edit")
          }
        }
      } catch (caught) {
        if (caught instanceof Error) {
          setError(caught.message)
        } else {
          throw caught
        }
      } finally {
        setLoading(false)
      }
    },
    [selected],
  )

  useEffect(() => {
    void refreshList(true)
  }, [refreshList])

  async function selectEnv(env: EnvMeta): Promise<void> {
    setLoading(true)
    setError("")
    try {
      const detail = await getEnv(env.name, env.version)
      setSelected(detail)
      setDraft(draftFromEnv(detail))
      setMode("edit")
    } catch (caught) {
      if (caught instanceof Error) {
        setError(caught.message)
      } else {
        throw caught
      }
    } finally {
      setLoading(false)
    }
  }

  async function saveDraft(): Promise<void> {
    setSaving(true)
    setError("")
    setMessage("")
    try {
      const payload = payloadFromDraft(draft)
      if (payload.name.length === 0 || payload.version.length === 0) {
        throw new Error("name and version are required")
      }
      if (mode === "create") {
        await createEnv(payload)
      } else {
        await updateEnv(payload)
      }
      const saved = await getEnv(payload.name, payload.version)
      setSelected(saved)
      setDraft(draftFromEnv(saved))
      setMode("edit")
      setMessage(`Saved ${saved.name}:${saved.version}`)
      await refreshList(false)
    } catch (caught) {
      if (caught instanceof Error) {
        setError(caught.message)
      } else {
        throw caught
      }
    } finally {
      setSaving(false)
    }
  }

  function startCreate(): void {
    setSelected(null)
    setDraft(emptyDraft())
    setMode("create")
    setMessage("")
    setError("")
  }

  const selectedKey = selected === null ? "" : `${selected.name}:${selected.version}`

  return (
    <main className="shell">
      <header className="topbar">
        <div>
          <p>EnvHub</p>
          <h1>Metadata Console</h1>
        </div>
        <button className="ghost-button" onClick={() => void refreshList(false)} type="button">
          Refresh
        </button>
      </header>

      {error ? <div className="alert error">{error}</div> : null}
      {message ? <div className="alert success">{message}</div> : null}

      <div className="workspace">
        <EnvList
          envs={envs}
          loading={loading}
          onSelect={(env) => void selectEnv(env)}
          selectedKey={selectedKey}
        />
        <div className="main-column">
          <EnvSummary env={selected} />
          <EnvEditor
            draft={draft}
            mode={mode}
            onChange={setDraft}
            onCreateNew={startCreate}
            onSave={() => void saveDraft()}
            saving={saving}
          />
        </div>
      </div>
    </main>
  )
}
