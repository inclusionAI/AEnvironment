import type { EnvMeta } from "../envSchema"

type EnvSummaryProps = {
  readonly env: EnvMeta | null
}

export function EnvSummary({ env }: EnvSummaryProps) {
  if (env === null) {
    return (
      <section className="summary-panel">
        <div className="section-title">
          <span>Overview</span>
        </div>
        <p className="muted">Select or create metadata to inspect it here.</p>
      </section>
    )
  }

  return (
    <section className="summary-panel">
      <div className="section-title">
        <span>Overview</span>
        <em>{env.status}</em>
      </div>
      <dl className="summary-grid">
        <div>
          <dt>Name</dt>
          <dd>{env.name}</dd>
        </div>
        <div>
          <dt>Version</dt>
          <dd>{env.version}</dd>
        </div>
        <div>
          <dt>Tags</dt>
          <dd>{env.tags.length > 0 ? env.tags.join(", ") : "-"}</dd>
        </div>
        <div>
          <dt>Code URL</dt>
          <dd>{env.codeUrl || "-"}</dd>
        </div>
        <div>
          <dt>Updated</dt>
          <dd>{env.updatedAt || "-"}</dd>
        </div>
        <div>
          <dt>Artifacts</dt>
          <dd>{env.artifacts.length}</dd>
        </div>
      </dl>
    </section>
  )
}
