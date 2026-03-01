import axios from 'axios'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || 'http://localhost:8080',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Response interceptor for error handling
api.interceptors.response.use(
  (response) => response,
  (error) => {
    console.error('API Error:', error.response?.data || error.message)
    return Promise.reject(error)
  }
)

// Health check
export const healthCheck = async () => {
  const response = await api.get('/health')
  return response.data
}

// Scan APIs
export const getScans = async (params?: { status?: string; page?: number; limit?: number }) => {
  const response = await api.get('/api/v1/scans', { params })
  return response.data
}

export const getScan = async (id: string) => {
  const response = await api.get(`/api/v1/scans/${id}`)
  return response.data
}

export const createScan = async (data: { target: string; type: string }) => {
  const response = await api.post('/api/v1/scans', data)
  return response.data
}

export const deleteScan = async (id: string) => {
  const response = await api.delete(`/api/v1/scans/${id}`)
  return response.data
}

// AI APIs
export const aiChat = async (data: { message: string; model?: string }) => {
  const response = await api.post('/api/v1/ai/chat', data)
  return response.data
}

export const aiValidate = async (data: { code: string; model?: string }) => {
  const response = await api.post('/api/v1/ai/validate', data)
  return response.data
}

export const aiReview = async (data: { code: string; model?: string }) => {
  const response = await api.post('/api/v1/ai/review', data)
  return response.data
}

// Etherscan APIs
export const getEthBalance = async (address: string) => {
  const response = await api.get(`/api/v1/etherscan/balance/${address}`)
  return response.data
}

export const getEthTransactions = async (address: string, params?: { page?: number; offset?: number }) => {
  const response = await api.get(`/api/v1/etherscan/tx/${address}`, { params })
  return response.data
}

export const getEthTokenBalance = async (address: string, tokenAddress: string) => {
  const response = await api.get(`/api/v1/etherscan/token/${address}/${tokenAddress}`)
  return response.data
}

export const getEthBlocks = async (params?: { page?: number; offset?: number }) => {
  const response = await api.get('/api/v1/etherscan/blocks', { params })
  return response.data
}

// GitHub APIs
export const getGithubRepos = async () => {
  const response = await api.get('/api/v1/github/repos')
  return response.data
}

export const getGithubWorkflows = async (repo: string) => {
  const response = await api.get(`/api/v1/github/workflows/${repo}`)
  return response.data
}

export const getGithubWorkflowRuns = async (repo: string, params?: { status?: string; per_page?: number }) => {
  const response = await api.get(`/api/v1/github/runs/${repo}`, { params })
  return response.data
}

export const getGithubPRs = async (repo: string, params?: { state?: string }) => {
  const response = await api.get(`/api/v1/github/prs/${repo}`, { params })
  return response.data
}

export const getGithubActions = async (repo: string, params?: { event?: string; per_page?: number }) => {
  const response = await api.get(`/api/v1/github/actions/${repo}`, { params })
  return response.data
}

export default api
