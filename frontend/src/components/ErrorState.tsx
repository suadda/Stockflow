import type { ApiError } from '../api/client'

interface Props {
  title: string
  error: ApiError
  onRetry: () => void
}

export function ErrorState({ title, error, onRetry }: Props) {
  const unreachable = error.code === 'NETWORK_ERROR' || (error.code === 'UNKNOWN' && error.status >= 500)
  return (
    <div className="error-state" role="alert">
      <p className="error-title">{title}</p>
      <p>{error.message}</p>
      {unreachable && <p className="muted">Check that the API is running on port 8080, then retry.</p>}
      <button type="button" className="btn" onClick={onRetry}>
        Retry
      </button>
    </div>
  )
}
