import { useState } from 'react'
import { aiChat, aiValidate, aiReview } from '../utils/api'

type Tab = 'chat' | 'validate' | 'review'

const models = [
  { value: 'gpt-4', label: 'GPT-4' },
  { value: 'gpt-3.5-turbo', label: 'GPT-3.5 Turbo' },
  { value: 'claude-3', label: 'Claude 3' },
  { value: 'gemini-pro', label: 'Gemini Pro' },
]

export default function AiValidation() {
  const [activeTab, setActiveTab] = useState<Tab>('chat')
  const [loading, setLoading] = useState(false)

  // Chat state
  const [chatMessages, setChatMessages] = useState<{ role: string; content: string }[]>([])
  const [chatInput, setChatInput] = useState('')
  const [chatModel, setChatModel] = useState('gpt-4')

  // Validate state
  const [validateCode, setValidateCode] = useState('')
  const [validateModel, setValidateModel] = useState('gpt-4')
  const [validateResult, setValidateResult] = useState<any>(null)

  // Review state
  const [reviewCode, setReviewCode] = useState('')
  const [reviewModel, setReviewModel] = useState('gpt-4')
  const [reviewResult, setReviewResult] = useState<any>(null)

  const handleChat = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!chatInput.trim()) return

    const userMessage = { role: 'user', content: chatInput }
    setChatMessages((prev) => [...prev, userMessage])
    setChatInput('')
    setLoading(true)

    try {
      const response = await aiChat({ message: chatInput, model: chatModel })
      setChatMessages((prev) => [...prev, response.message])
    } catch (error) {
      console.error('Chat error:', error)
      setChatMessages((prev) => [
        ...prev,
        { role: 'assistant', content: 'Error: Failed to get response' },
      ])
    } finally {
      setLoading(false)
    }
  }

  const handleValidate = async () => {
    if (!validateCode.trim()) return
    setLoading(true)
    try {
      const response = await aiValidate({ code: validateCode, model: validateModel })
      setValidateResult(response)
    } catch (error) {
      console.error('Validate error:', error)
      setValidateResult({ error: 'Failed to validate code' })
    } finally {
      setLoading(false)
    }
  }

  const handleReview = async () => {
    if (!reviewCode.trim()) return
    setLoading(true)
    try {
      const response = await aiReview({ code: reviewCode, model: reviewModel })
      setReviewResult(response)
    } catch (error) {
      console.error('Review error:', error)
      setReviewResult({ error: 'Failed to review code' })
    } finally {
      setLoading(false)
    }
  }

  const tabs = [
    { id: 'chat' as const, label: 'AI Chat' },
    { id: 'validate' as const, label: 'Code Validation' },
    { id: 'review' as const, label: 'Code Review' },
  ]

  return (
    <div className="space-y-6">
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
          {activeTab === 'chat' && (
            <div className="space-y-4">
              <div className="flex items-center gap-4 mb-4">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                  Model:
                </label>
                <select
                  value={chatModel}
                  onChange={(e) => setChatModel(e.target.value)}
                  className="input w-auto"
                >
                  {models.map((model) => (
                    <option key={model.value} value={model.value}>
                      {model.label}
                    </option>
                  ))}
                </select>
              </div>

              <div className="border border-gray-200 dark:border-gray-700 rounded-lg h-96 overflow-y-auto p-4 space-y-4">
                {chatMessages.length === 0 ? (
                  <p className="text-center text-gray-500 dark:text-gray-400">
                    Start a conversation with the AI
                  </p>
                ) : (
                  chatMessages.map((msg, idx) => (
                    <div
                      key={idx}
                      className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}
                    >
                      <div
                        className={`max-w-[80%] rounded-lg px-4 py-2 ${
                          msg.role === 'user'
                            ? 'bg-primary-600 text-white'
                            : 'bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white'
                        }`}
                      >
                        <p className="text-sm whitespace-pre-wrap">{msg.content}</p>
                      </div>
                    </div>
                  ))
                )}
                {loading && (
                  <div className="flex justify-start">
                    <div className="bg-gray-100 dark:bg-gray-700 rounded-lg px-4 py-2">
                      <div className="flex gap-1">
                        <div className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" />
                        <div className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: '0.1s' }} />
                        <div className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: '0.2s' }} />
                      </div>
                    </div>
                  </div>
                )}
              </div>

              <form onSubmit={handleChat} className="flex gap-3">
                <input
                  type="text"
                  value={chatInput}
                  onChange={(e) => setChatInput(e.target.value)}
                  placeholder="Ask the AI..."
                  className="input flex-1"
                  disabled={loading}
                />
                <button
                  type="submit"
                  disabled={loading || !chatInput.trim()}
                  className="px-6 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 transition-colors"
                >
                  Send
                </button>
              </form>
            </div>
          )}

          {activeTab === 'validate' && (
            <div className="space-y-4">
              <div className="flex items-center gap-4 mb-4">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                  Model:
                </label>
                <select
                  value={validateModel}
                  onChange={(e) => setValidateModel(e.target.value)}
                  className="input w-auto"
                >
                  {models.map((model) => (
                    <option key={model.value} value={model.value}>
                      {model.label}
                    </option>
                  ))}
                </select>
              </div>

              <textarea
                value={validateCode}
                onChange={(e) => setValidateCode(e.target.value)}
                placeholder="Paste your code here to validate..."
                className="input h-48 font-mono text-sm"
              />

              <button
                onClick={handleValidate}
                disabled={loading || !validateCode.trim()}
                className="px-6 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 transition-colors"
              >
                {loading ? 'Validating...' : 'Validate Code'}
              </button>

              {validateResult && (
                <div className="mt-6 p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
                  {validateResult.error ? (
                    <p className="text-red-600 dark:text-red-400">{validateResult.error}</p>
                  ) : (
                    <div className="space-y-3">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
                          Valid:
                        </span>
                        <span
                          className={`px-2 py-1 rounded text-xs font-medium ${
                            validateResult.valid
                              ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                              : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                          }`}
                        >
                          {validateResult.valid ? 'Yes' : 'No'}
                        </span>
                      </div>
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
                          Score:
                        </span>
                        <span className="text-sm text-gray-900 dark:text-white">
                          {validateResult.score}/100
                        </span>
                      </div>
                      {validateResult.issues && validateResult.issues.length > 0 && (
                        <div>
                          <p className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                            Issues:
                          </p>
                          <ul className="space-y-2">
                            {validateResult.issues.map((issue: any, idx: number) => (
                              <li
                                key={idx}
                                className={`text-sm px-3 py-2 rounded ${
                                  issue.severity === 'error'
                                    ? 'bg-red-50 dark:bg-red-900/30 text-red-700 dark:text-red-300'
                                    : issue.severity === 'warning'
                                    ? 'bg-yellow-50 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-300'
                                    : 'bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300'
                                }`}
                              >
                                {issue.message}
                              </li>
                            ))}
                          </ul>
                        </div>
                      )}
                    </div>
                  )}
                </div>
              )}
            </div>
          )}

          {activeTab === 'review' && (
            <div className="space-y-4">
              <div className="flex items-center gap-4 mb-4">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                  Model:
                </label>
                <select
                  value={reviewModel}
                  onChange={(e) => setReviewModel(e.target.value)}
                  className="input w-auto"
                >
                  {models.map((model) => (
                    <option key={model.value} value={model.value}>
                      {model.label}
                    </option>
                  ))}
                </select>
              </div>

              <textarea
                value={reviewCode}
                onChange={(e) => setReviewCode(e.target.value)}
                placeholder="Paste your code here for review..."
                className="input h-48 font-mono text-sm"
              />

              <button
                onClick={handleReview}
                disabled={loading || !reviewCode.trim()}
                className="px-6 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 transition-colors"
              >
                {loading ? 'Reviewing...' : 'Review Code'}
              </button>

              {reviewResult && (
                <div className="mt-6 p-4 bg-gray-50 dark:bg-gray-700 rounded-lg space-y-4">
                  {reviewResult.error ? (
                    <p className="text-red-600 dark:text-red-400">{reviewResult.error}</p>
                  ) : (
                    <>
                      <div>
                        <p className="text-sm font-medium text-gray-700 dark:text-gray-300">
                          Score: {reviewResult.score}/100
                        </p>
                      </div>
                      <div>
                        <p className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                          Summary:
                        </p>
                        <p className="text-sm text-gray-900 dark:text-white whitespace-pre-wrap">
                          {reviewResult.summary}
                        </p>
                      </div>
                      {reviewResult.suggestions && reviewResult.suggestions.length > 0 && (
                        <div>
                          <p className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                            Suggestions:
                          </p>
                          <ul className="space-y-2">
                            {reviewResult.suggestions.map((suggestion: string, idx: number) => (
                              <li
                                key={idx}
                                className="text-sm text-gray-900 dark:text-white bg-white dark:bg-gray-800 px-3 py-2 rounded"
                              >
                                {suggestion}
                              </li>
                            ))}
                          </ul>
                        </div>
                      )}
                    </>
                  )}
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
