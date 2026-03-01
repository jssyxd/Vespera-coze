import { useState } from 'react'
import { useFetch, usePolling } from '../hooks/useApi'
import {
  getGithubRepos,
  getGithubWorkflowRuns,
  getGithubPRs,
  getGithubActions,
} from '../utils/api'
import Loading from '../components/Loading'
import ErrorMessage from '../components/ErrorMessage'

type Tab = 'repos' | 'workflows' | 'prs' | 'actions'

export default function Github() {
  const [activeTab, setActiveTab] = useState<Tab>('repos')
  const [selectedRepo, setSelectedRepo] = useState('')

  const { data: reposData, loading: reposLoading, error: reposError, refetch: refetchRepos } = useFetch(
    () => getGithubRepos()
  )

  const { data: workflowsData, loading: workflowsLoading, error: workflowsError, refetch: refetchWorkflows } = usePolling(
    () => getGithubWorkflowRuns(selectedRepo, { per_page: 10 }),
    10000
  )

  const { data: prsData, loading: prsLoading, error: prsError, refetch: refetchPRs } = useFetch(
    () => getGithubPRs(selectedRepo, { state: 'open' })
  )

  const { data: actionsData, loading: actionsLoading, error: actionsError, refetch: refetchActions } = useFetch(
    () => getGithubActions(selectedRepo, { per_page: 10 })
  )

  const repos = reposData?.data || []

  const handleRepoChange = (repo: string) => {
    setSelectedRepo(repo)
  }

  const tabs = [
    { id: 'repos' as const, label: 'Repositories' },
    { id: 'workflows' as const, label: 'CI/CD Status' },
    { id: 'prs' as const, label: 'Pull Requests' },
    { id: 'actions' as const, label: 'Actions' },
  ]

  const getStatusColor = (status: string, conclusion?: string) => {
    if (status === 'completed' || status === 'in_progress') {
      return conclusion === 'success'
        ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
        : conclusion === 'failure'
        ? 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
        : conclusion === 'cancelled'
        ? 'bg-gray-100 text-gray-800 dark:bg-gray-600 dark:text-gray-200'
        : 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200'
    }
    return 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200'
  }

  return (
    <div className="space-y-6">
      {repos.length > 0 && (
        <div className="flex items-center gap-4">
          <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
            Repository:
          </label>
          <select
            value={selectedRepo}
            onChange={(e) => handleRepoChange(e.target.value)}
            className="input w-auto"
          >
            <option value="">Select a repository</option>
            {repos.map((repo: any) => (
              <option key={repo.name} value={repo.name}>
                {repo.full_name}
              </option>
            ))}
          </select>
        </div>
      )}

      <div className="bg-white dark:bg-gray-800 rounded-lg shadow-md">
        <div className="border-b border-gray-200 dark:border-gray-700">
          <nav className="flex space-x-8 px-6" aria-label="Tabs">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`py-4 px-1 border-b-2 font-medium text-sm transition-colors ${
                  activeTab === tab.id
                    ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                    : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'
                }`}
              >
                {tab.label}
              </button>
            ))}
          </nav>
        </div>

        <div className="p-6">
          {activeTab === 'repos' && (
            <div>
              {reposLoading ? (
                <Loading />
              ) : reposError ? (
                <ErrorMessage message={reposError} onRetry={refetchRepos} />
              ) : repos.length > 0 ? (
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                  {repos.map((repo: any) => (
                    <div
                      key={repo.name}
                      className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4 hover:shadow-md transition-shadow cursor-pointer"
                      onClick={() => handleRepoChange(repo.name)}
                    >
                      <div className="flex items-start justify-between">
                        <div>
                          <h3 className="font-medium text-gray-900 dark:text-white">
                            {repo.name}
                          </h3>
                          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                            {repo.description || 'No description'}
                          </p>
                        </div>
                        <span
                          className={`px-2 py-1 rounded text-xs font-medium ${
                            repo.visibility === 'public'
                              ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                              : 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200'
                          }`}
                        >
                          {repo.visibility}
                        </span>
                      </div>
                      <div className="flex items-center gap-4 mt-4 text-xs text-gray-500 dark:text-gray-400">
                        <span className="flex items-center gap-1">
                          <span className="w-2 h-2 rounded-full bg-yellow-400" />
                          {repo.language}
                        </span>
                        <span>⭐ {repo.stars}</span>
                        <span>{repo.default_branch}</span>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <p className="text-gray-500 dark:text-gray-400">No repositories found</p>
              )}
            </div>
          )}

          {activeTab === 'workflows' && (
            <div>
              {!selectedRepo ? (
                <p className="text-gray-500 dark:text-gray-400">
                  Select a repository to view workflows
                </p>
              ) : workflowsLoading ? (
                <Loading />
              ) : workflowsError ? (
                <ErrorMessage message={workflowsError} onRetry={refetchWorkflows} />
              ) : workflowsData?.data && workflowsData.data.length > 0 ? (
                <div className="space-y-3">
                  {workflowsData.data.map((run: any) => (
                    <div
                      key={run.id}
                      className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4"
                    >
                      <div className="flex items-start justify-between">
                        <div>
                          <p className="font-medium text-gray-900 dark:text-white">
                            {run.name}
                          </p>
                          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                            Branch: {run.branch} • Commit: {run.commit?.slice(0, 7)}
                          </p>
                          <p className="text-xs text-gray-500 dark:text-gray-400">
                            {new Date(run.created_at).toLocaleString()}
                          </p>
                        </div>
                        <span
                          className={`px-3 py-1 rounded text-sm font-medium ${getStatusColor(
                            run.status,
                            run.conclusion
                          )}`}
                        >
                          {run.conclusion || run.status}
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <p className="text-gray-500 dark:text-gray-400">No workflow runs found</p>
              )}
            </div>
          )}

          {activeTab === 'prs' && (
            <div>
              {!selectedRepo ? (
                <p className="text-gray-500 dark:text-gray-400">
                  Select a repository to view pull requests
                </p>
              ) : prsLoading ? (
                <Loading />
              ) : prsError ? (
                <ErrorMessage message={prsError} onRetry={refetchPRs} />
              ) : prsData?.data && prsData.data.length > 0 ? (
                <div className="space-y-3">
                  {prsData.data.map((pr: any) => (
                    <div
                      key={pr.id}
                      className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4"
                    >
                      <div className="flex items-start justify-between">
                        <div className="flex items-start gap-3">
                          <img
                            src={pr.user.avatar_url}
                            alt={pr.user.login}
                            className="w-10 h-10 rounded-full"
                          />
                          <div>
                            <p className="font-medium text-gray-900 dark:text-white">
                              #{pr.number} {pr.title}
                            </p>
                            <p className="text-sm text-gray-500 dark:text-gray-400">
                              by {pr.user.login} • {new Date(pr.created_at).toLocaleString()}
                            </p>
                          </div>
                        </div>
                        <span
                          className={`px-3 py-1 rounded text-sm font-medium ${
                            pr.state === 'open'
                              ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                              : pr.merged_at
                              ? 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200'
                              : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                          }`}
                        >
                          {pr.merged_at ? 'merged' : pr.state}
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <p className="text-gray-500 dark:text-gray-400">No pull requests found</p>
              )}
            </div>
          )}

          {activeTab === 'actions' && (
            <div>
              {!selectedRepo ? (
                <p className="text-gray-500 dark:text-gray-400">
                  Select a repository to view actions
                </p>
              ) : actionsLoading ? (
                <Loading />
              ) : actionsError ? (
                <ErrorMessage message={actionsError} onRetry={refetchActions} />
              ) : actionsData?.data && actionsData.data.length > 0 ? (
                <div className="space-y-3">
                  {actionsData.data.map((action: any) => (
                    <div
                      key={action.id}
                      className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4"
                    >
                      <div className="flex items-start justify-between">
                        <div>
                          <p className="font-medium text-gray-900 dark:text-white">
                            {action.name}
                          </p>
                          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                            Event: {action.event} • Branch: {action.branch}
                          </p>
                          <p className="text-xs text-gray-500 dark:text-gray-400">
                            {new Date(action.created_at).toLocaleString()}
                          </p>
                        </div>
                        <span
                          className={`px-3 py-1 rounded text-sm font-medium ${getStatusColor(
                            action.status,
                            action.conclusion
                          )}`}
                        >
                          {action.conclusion || action.status}
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <p className="text-gray-500 dark:text-gray-400">No actions found</p>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
