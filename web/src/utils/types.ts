// Health
export interface HealthStatus {
  status: string
  uptime: number
  timestamp: string
}

// Scan
export interface Scan {
  id: string
  target: string
  type: string
  status: 'pending' | 'running' | 'completed' | 'failed'
  result?: ScanResult
  created_at: string
  updated_at: string
}

export interface ScanResult {
  vulnerabilities: Vulnerability[]
  summary: ScanSummary
}

export interface Vulnerability {
  id: string
  severity: 'critical' | 'high' | 'medium' | 'low' | 'info'
  title: string
  description: string
  file?: string
  line?: number
}

export interface ScanSummary {
  critical: number
  high: number
  medium: number
  low: number
  info: number
}

// AI
export interface AIChatMessage {
  role: 'user' | 'assistant'
  content: string
}

export interface AIChatResponse {
  message: AIChatMessage
  model: string
}

export interface AIValidationResult {
  valid: boolean
  issues: AIValidationIssue[]
  score: number
}

export interface AIValidationIssue {
  severity: 'error' | 'warning' | 'info'
  message: string
  line?: number
}

export interface AIReviewResult {
  summary: string
  suggestions: string[]
  score: number
}

// Etherscan
export interface EthBalance {
  address: string
  balance: string
  balance_wei: string
}

export interface EthTransaction {
  hash: string
  from: string
  to: string
  value: string
  gas_used: string
  gas_price: string
  timestamp: string
  block_number: number
  status: 'success' | 'failed'
}

export interface EthTokenBalance {
  address: string
  token: string
  balance: string
  decimals: number
}

export interface EthBlock {
  number: number
  hash: string
  timestamp: string
  transactions: number
  gas_used: string
  miner: string
}

// GitHub
export interface GithubRepo {
  name: string
  full_name: string
  description: string
  url: string
  default_branch: string
  visibility: 'public' | 'private'
  language: string
  stars: number
}

export interface GithubWorkflow {
  id: number
  name: string
  path: string
  state: 'active' | 'disabled'
}

export interface GithubWorkflowRun {
  id: number
  name: string
  status: 'queued' | 'in_progress' | 'completed'
  conclusion?: 'success' | 'failure' | 'cancelled' | 'skipped'
  branch: string
  commit: string
  created_at: string
  updated_at: string
}

export interface GithubPR {
  id: number
  number: number
  title: string
  state: 'open' | 'closed'
  user: {
    login: string
    avatar_url: string
  }
  created_at: string
  updated_at: string
  merged_at?: string
}

export interface GithubAction {
  id: number
  name: string
  event: string
  status: 'queued' | 'in_progress' | 'completed'
  conclusion?: string
  branch: string
  commit: string
  created_at: string
}
