export function LoadMessage({ loading, error, retry }: { loading: boolean; error?: string; retry: () => void }) {
  return loading ? <p role="status">Loading…</p> : error ? <div className="manage-feedback" role="alert"><p>{error}</p><button type="button" onClick={retry}>Retry</button></div> : null
}
export function Feedback({ error, message }: { error: string; message: string }) {
  return <>{error && <p className="manage-feedback manage-error" role="alert">{error}</p>}{message && <p className="manage-feedback" role="status">{message}</p>}</>
}
export function Pagination({ page, size, total, change, busy = false }: { page: number; size: number; total: number; change: (page: number) => void; busy?: boolean }) {
  if (total <= size && page === 1) return null
  return <nav className="manage-pagination" aria-label="Pagination"><button type="button" disabled={page <= 1 || busy} onClick={() => change(page - 1)}>Previous</button><span>Page {page} of {Math.max(1, Math.ceil(total / size))} · {total} results</span><button type="button" disabled={page * size >= total || busy} onClick={() => change(page + 1)}>Next</button></nav>
}
