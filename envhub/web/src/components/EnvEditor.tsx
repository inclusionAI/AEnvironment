import type { ChangeEvent } from "react"
import type { EnvDraft } from "../draft"

type EnvEditorProps = {
  readonly draft: EnvDraft
  readonly mode: "create" | "edit"
  readonly saving: boolean
  readonly onChange: (draft: EnvDraft) => void
  readonly onSave: () => void
  readonly onCreateNew: () => void
}

const STATUS_OPTIONS = [
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

export function EnvEditor({ draft, mode, saving, onChange, onSave, onCreateNew }: EnvEditorProps) {
  const lockedIdentity = mode === "edit"

  function setField(field: keyof EnvDraft) {
    return (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>) => {
      onChange({ ...draft, [field]: event.target.value })
    }
  }

  return (
    <section className="editor-panel" aria-label="Metadata editor">
      <div className="editor-head">
        <div className="section-title">
          <span>{mode === "create" ? "Create Meta" : "Edit Meta"}</span>
          <em>{draft.name && draft.version ? `${draft.name}:${draft.version}` : "draft"}</em>
        </div>
        <div className="actions">
          <button className="ghost-button" onClick={onCreateNew} type="button">
            New
          </button>
          <button className="primary-button" disabled={saving} onClick={onSave} type="button">
            {saving ? "Saving..." : "Save Meta"}
          </button>
        </div>
      </div>

      <div className="form-grid">
        <label>
          Name
          <input disabled={lockedIdentity} onChange={setField("name")} value={draft.name} />
        </label>
        <label>
          Version
          <input disabled={lockedIdentity} onChange={setField("version")} value={draft.version} />
        </label>
        <label>
          Status
          <select onChange={setField("status")} value={draft.status}>
            {STATUS_OPTIONS.map((status) => (
              <option key={status} value={status}>
                {status}
              </option>
            ))}
          </select>
        </label>
        <label>
          Tags
          <input onChange={setField("tags")} placeholder="linux, python" value={draft.tags} />
        </label>
      </div>

      <label className="wide-field">
        Description
        <textarea onChange={setField("description")} rows={3} value={draft.description} />
      </label>

      <label className="wide-field">
        Code URL
        <input onChange={setField("codeUrl")} value={draft.codeUrl} />
      </label>

      <div className="json-grid">
        <JsonField
          label="Build Config"
          onChange={setField("buildConfigJson")}
          value={draft.buildConfigJson}
        />
        <JsonField
          label="Test Config"
          onChange={setField("testConfigJson")}
          value={draft.testConfigJson}
        />
        <JsonField
          label="Deploy Config"
          onChange={setField("deployConfigJson")}
          value={draft.deployConfigJson}
        />
        <JsonField
          label="Artifacts"
          onChange={setField("artifactsJson")}
          value={draft.artifactsJson}
        />
      </div>
    </section>
  )
}

type JsonFieldProps = {
  readonly label: string
  readonly value: string
  readonly onChange: (event: ChangeEvent<HTMLTextAreaElement>) => void
}

function JsonField({ label, value, onChange }: JsonFieldProps) {
  return (
    <label className="json-field">
      {label}
      <textarea spellCheck={false} onChange={onChange} rows={8} value={value} />
    </label>
  )
}
