import { useState, useEffect } from 'react'
import { healthCheck } from '../utils/api'
import type { HealthStatus } from '../utils/types'

interface HeaderProps {
  title: string
}

export default function Header({ title }: HeaderProps) {
  const [health, setHealth] = useState<HealthStatus | null>(null)
  const [currentTime, setCurrentTime] = useState(new Date())

  useEffect(() => {
    const fetchHealth = async () => {
      try {
        const data = await healthCheck()
        setHealth(data)
      } catch (error) {
        console.error('Health check failed:', error)
      }
    }
    fetchHealth()
    const timer = setInterval(fetchHealth, 30000)
    return () => clearInterval(timer)
  }, [])

  useEffect(() => {
    const timer = setInterval(() => setCurrentTime(new Date()), 1000)
    return () => clearInterval(timer)
  }, [])

  return (
    <header className="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-6 py-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">{title}</h1>
        <div className="flex items-center gap-6">
          <div className="flex items-center gap-2">
            <span
              className={`w-2 h-2 rounded-full ${
                health?.status === 'ok' ? 'bg-green-500' : 'bg-red-500'
              }`}
            />
            <span className="text-sm text-gray-600 dark:text-gray-400">
              API: {health?.status || 'Unknown'}
            </span>
          </div>
          <div className="text-sm text-gray-600 dark:text-gray-400">
            {currentTime.toLocaleTimeString()}
          </div>
        </div>
      </div>
    </header>
  )
}
