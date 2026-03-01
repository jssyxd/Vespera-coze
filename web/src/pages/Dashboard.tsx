import { useFetch } from '../hooks/useApi'
import { getScans, getGithubRepos } from '../utils/api'
import StatCard from '../components/StatCard'
import Loading from '../components/Loading'
import ErrorMessage from '../components/ErrorMessage'

export default function Dashboard() {
  const { data: scansData, loading: scansLoading, error: scansError, refetch: refetchScans } = useFetch(
    () => getScans({ limit: 100 })
  )
  const { data: reposData, loading: reposLoading, error: reposError, refetch: refetchRepos } = useFetch(
    () => getGithubRepos()
  )

  if (scansLoading || reposLoading) {
    return <Loading />
  }

  if (scansError || reposError) {
    return (
      <ErrorMessage
        message={scansError || reposError || 'Failed to load dashboard data'}
        onRetry={() => {
          refetchScans()
          refetchRepos()
        }}
      />
    )
  }

  const scans = scansData?.data || []
  const repos = reposData?.data || []

  const stats = {
    totalScans: scans.length,
    completedScans: scans.filter((s: { status: string }) => s.status === 'completed').length,
    failedScans: scans.filter((s: { status: string }) => s.status === 'failed').length,
    activeScans: scans.filter((s: { status: string }) => s.status === 'running').length,
    totalRepos: repos.length,
    publicRepos: repos.filter((r: { visibility: string }) => r.visibility === 'public').length,
  }

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <StatCard
          title="Total Scans"
          value={stats.totalScans}
          color="primary"
          icon={
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
            </svg>
          }
        />
        <StatCard
          title="Completed"
          value={stats.completedScans}
          color="green"
          icon={
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          }
        />
        <StatCard
          title="Failed"
          value={stats.failedScans}
          color="red"
          icon={
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          }
        />
        <StatCard
          title="Active"
          value={stats.activeScans}
          color="yellow"
          icon={
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
            </svg>
          }
        />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="bg-white dark:bg-gray-800 rounded-6">
          <h2 className="text-lg font-semibold text-lg shadow-md p-gray-900 dark:text-white mb-4">
            Recent Scans
          </h2>
          {scans.length > 0 ? (
            <div className="space-y-3">
              {scans.slice(0, 5).map((scan: { id: string; target: string; status: string; created_at: string }) => (
                <div
                  key={scan.id}
                  className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg"
                >
                  <div>
                    <p className="font-medium text-gray-900 dark:text-white">{scan.target}</p>
                    <p className="text-sm text-gray-500 dark:text-gray-400">
                      {new Date(scan.created_at).toLocaleString()}
                    </p>
                  </div>
                  <span
                    className={`px-3 py-1 rounded-full text-xs font-medium ${
                      scan.status === 'completed'
                        ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                        : scan.status === 'failed'
                        ? 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                        : scan.status === 'running'
                        ? 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200'
                        : 'bg-gray-100 text-gray-800 dark:bg-gray-600 dark:text-gray-200'
                    }`}
                  >
                    {scan.status}
                  </span>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-gray-500 dark:text-gray-400">No scans yet</p>
          )}
        </div>

        <div className="bg-white dark:bg-gray-800 rounded-lg shadow-md p-6">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            GitHub Repositories
          </h2>
          {repos.length > 0 ? (
            <div className="space-y-3">
              {repos.slice(0, 5).map((repo: { name: string; full_name: string; language: string; stars: number }) => (
                <div
                  key={repo.name}
                  className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg"
                >
                  <div>
                    <p className="font-medium text-gray-900 dark:text-white">{repo.name}</p>
                    <p className="text-sm text-gray-500 dark:text-gray-400">
                      {repo.language} • {repo.stars} stars
                    </p>
                  </div>
                  <a
                    href={repo.full_name}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary-600 hover:text-primary-700"
                  >
                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                    </svg>
                  </a>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-gray-500 dark:text-gray-400">No repositories</p>
          )}
        </div>
      </div>
    </div>
  )
}
