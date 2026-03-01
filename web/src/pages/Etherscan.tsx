import { useState } from 'react'
import { useFetch } from '../hooks/useApi'
import {
  getEthBalance,
  getEthTransactions,
  getEthBlocks,
} from '../utils/api'
import Loading from '../components/Loading'
import ErrorMessage from '../components/ErrorMessage'

type Tab = 'balance' | 'transactions' | 'tokens' | 'blocks'

const sampleAddress = '0x742d35Cc6634C0532925a3b844Bc9e7595f0eB1E'

export default function Etherscan() {
  const [activeTab, setActiveTab] = useState<Tab>('balance')
  const [address, setAddress] = useState(sampleAddress)
  const [searchedAddress, setSearchedAddress] = useState(sampleAddress)

  const { data: balanceData, loading: balanceLoading, error: balanceError, refetch: refetchBalance } = useFetch(
    () => getEthBalance(searchedAddress)
  )

  const { data: txData, loading: txLoading, error: txError, refetch: refetchTx } = useFetch(
    () => getEthTransactions(searchedAddress, { page: 1, offset: 20 })
  )

  const { data: blocksData, loading: blocksLoading, error: blocksError, refetch: refetchBlocks } = useFetch(
    () => getEthBlocks({ page: 1, offset: 10 })
  )

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault()
    setSearchedAddress(address)
  }

  const formatEth = (wei: string) => {
    try {
      return (parseFloat(wei) / 1e18).toFixed(6)
    } catch {
      return wei
    }
  }

  const formatAddress = (addr: string) => {
    return addr.slice(0, 6) + '...' + addr.slice(-4)
  }

  const tabs = [
    { id: 'balance' as const, label: 'Balance' },
    { id: 'transactions' as const, label: 'Transactions' },
    { id: 'tokens' as const, label: 'Tokens' },
    { id: 'blocks' as const, label: 'Blocks' },
  ]

  return (
    <div className="space-y-6">
      <form onSubmit={handleSearch} className="flex gap-3">
        <input
          type="text"
          value={address}
          onChange={(e) => setAddress(e.target.value)}
          placeholder="Enter Ethereum address"
          className="input flex-1"
        />
        <button
          type="submit"
          className="px-6 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
        >
          Search
        </button>
      </form>

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
          {activeTab === 'balance' && (
            <div>
              {balanceLoading ? (
                <Loading />
              ) : balanceError ? (
                <ErrorMessage message={balanceError} onRetry={refetchBalance} />
              ) : balanceData ? (
                <div className="space-y-4">
                  <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-6">
                    <p className="text-sm text-gray-500 dark:text-gray-400 mb-1">Address</p>
                    <p className="font-mono text-lg text-gray-900 dark:text-white">
                      {balanceData.address}
                    </p>
                  </div>
                  <div className="grid grid-cols-2 gap-4">
                    <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-6">
                      <p className="text-sm text-gray-500 dark:text-gray-400 mb-1">Balance (ETH)</p>
                      <p className="text-3xl font-bold text-gray-900 dark:text-white">
                        {formatEth(balanceData.balance)}
                      </p>
                    </div>
                    <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-6">
                      <p className="text-sm text-gray-500 dark:text-gray-400 mb-1">Balance (Wei)</p>
                      <p className="font-mono text-lg text-gray-900 dark:text-white">
                        {balanceData.balance_wei}
                      </p>
                    </div>
                  </div>
                </div>
              ) : (
                <p className="text-gray-500">No data available</p>
              )}
            </div>
          )}

          {activeTab === 'transactions' && (
            <div>
              {txLoading ? (
                <Loading />
              ) : txError ? (
                <ErrorMessage message={txError} onRetry={refetchTx} />
              ) : txData?.data && txData.data.length > 0 ? (
                <div className="space-y-3">
                  {txData.data.map((tx: any) => (
                    <div
                      key={tx.hash}
                      className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4"
                    >
                      <div className="flex items-start justify-between">
                        <div className="space-y-1">
                          <p className="font-mono text-sm text-gray-900 dark:text-white">
                            {formatAddress(tx.hash)}
                          </p>
                          <p className="text-xs text-gray-500 dark:text-gray-400">
                            From: {formatAddress(tx.from)} → To: {formatAddress(tx.to)}
                          </p>
                          <p className="text-xs text-gray-500 dark:text-gray-400">
                            Block: {tx.block_number} • {new Date(tx.timestamp).toLocaleString()}
                          </p>
                        </div>
                        <div className="text-right">
                          <p className="font-medium text-gray-900 dark:text-white">
                            {formatEth(tx.value)} ETH
                          </p>
                          <span
                            className={`px-2 py-1 rounded text-xs font-medium ${
                              tx.status === 'success'
                                ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                                : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                            }`}
                          >
                            {tx.status}
                          </span>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <p className="text-gray-500 dark:text-gray-400">No transactions found</p>
              )}
            </div>
          )}

          {activeTab === 'tokens' && (
            <div>
              <p className="text-gray-500 dark:text-gray-400 mb-4">
                Token balance lookup. Enter a token address to check balances.
              </p>
              <div className="flex gap-3">
                <input
                  type="text"
                  placeholder="Token contract address (optional)"
                  className="input flex-1"
                />
                <button className="px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700">
                  Check
                </button>
              </div>
            </div>
          )}

          {activeTab === 'blocks' && (
            <div>
              {blocksLoading ? (
                <Loading />
              ) : blocksError ? (
                <ErrorMessage message={blocksError} onRetry={refetchBlocks} />
              ) : blocksData?.data && blocksData.data.length > 0 ? (
                <div className="space-y-3">
                  {blocksData.data.map((block: any) => (
                    <div
                      key={block.number}
                      className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4"
                    >
                      <div className="flex items-start justify-between">
                        <div>
                          <p className="font-medium text-gray-900 dark:text-white">
                            Block #{block.number}
                          </p>
                          <p className="text-xs text-gray-500 dark:text-gray-400 font-mono">
                            {block.hash}
                          </p>
                          <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                            Miner: {formatAddress(block.miner)} • {new Date(block.timestamp).toLocaleString()}
                          </p>
                        </div>
                        <div className="text-right">
                          <p className="text-sm text-gray-900 dark:text-white">
                            {block.transactions} txns
                          </p>
                          <p className="text-xs text-gray-500 dark:text-gray-400">
                            Gas: {block.gas_used}
                          </p>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <p className="text-gray-500 dark:text-gray-400">No blocks found</p>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
