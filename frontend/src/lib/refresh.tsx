import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react'

interface Refresh {
  /** Increments after every successful mutation. */
  version: number
  bump: () => void
}

const RefreshContext = createContext<Refresh>({ version: 0, bump: () => {} })

/** Lists and summary cards depend on `version`, so any mutation that calls
 * bump() makes them re-fetch from the server. */
export function RefreshProvider({ children }: { children: ReactNode }) {
  const [version, setVersion] = useState(0)
  const bump = useCallback(() => setVersion((v) => v + 1), [])
  const value = useMemo(() => ({ version, bump }), [version, bump])
  return <RefreshContext.Provider value={value}>{children}</RefreshContext.Provider>
}

export function useRefresh(): Refresh {
  return useContext(RefreshContext)
}
