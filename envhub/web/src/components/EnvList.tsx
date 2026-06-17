import type { EnvMeta } from "../envSchema"

type EnvListProps = {
  readonly envs: readonly EnvMeta[]
  readonly selectedKey: string
  readonly loading: boolean
  readonly onSelect: (env: EnvMeta) => void
}

export function EnvList({ envs, selectedKey, loading, onSelect }: EnvListProps) {
  return (
    <section className="env-list" aria-label="Environment list">
      <div className="section-title">
        <span>Environments</span>
        <strong>{envs.length}</strong>
      </div>
      {loading ? <p className="muted">Loading metadata...</p> : null}
      <div className="list-scroll">
        {envs.map((env) => {
          const key = `${env.name}:${env.version}`
          return (
            <button
              className={key === selectedKey ? "env-row active" : "env-row"}
              key={key}
              onClick={() => onSelect(env)}
              type="button"
            >
              <span>
                <strong>{env.name}</strong>
                <small>{env.description || "No description"}</small>
              </span>
              <span className="row-meta">
                <small>{env.version}</small>
                <em>{env.status}</em>
              </span>
            </button>
          )
        })}
        {envs.length === 0 && !loading ? (
          <p className="muted">No environment metadata yet.</p>
        ) : null}
      </div>
    </section>
  )
}
