import { useState, useEffect, useCallback } from 'react'

interface UseFetchState<T> {
  data: T | null
  loading: boolean
  error: string | null
}

export function useFetch<T>(fetcher: () => Promise<T>) {
  const [state, setState] = useState<UseFetchState<T>>({
    data: null,
    loading: true,
    error: null,
  })

  const fetch = useCallback(async () => {
    setState({ data: null, loading: true, error: null })
    try {
      const data = await fetcher()
      setState({ data, loading: false, error: null })
    } catch (err) {
      setState({
        data: null,
        loading: false,
        error: err instanceof Error ? err.message : 'An error occurred',
      })
    }
  }, [fetcher])

  useEffect(() => {
    fetch()
  }, [fetch])

  return { ...state, refetch: fetch }
}

export function usePolling<T>(
  fetcher: () => Promise<T>,
  interval: number
) {
  const [state, setState] = useState<UseFetchState<T>>({
    data: null,
    loading: true,
    error: null,
  })

  const fetch = useCallback(async () => {
    try {
      const data = await fetcher()
      setState({ data, loading: false, error: null })
    } catch (err) {
      setState({
        data: null,
        loading: false,
        error: err instanceof Error ? err.message : 'An error occurred',
      })
    }
  }, [fetcher])

  useEffect(() => {
    fetch()
    const timer = setInterval(fetch, interval)
    return () => clearInterval(timer)
  }, [fetch, interval])

  return { ...state, refetch: fetch }
}
