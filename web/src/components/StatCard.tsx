interface StatCardProps {
  title: string
  value: string | number
  icon: React.ReactNode
  trend?: {
    value: number
    isPositive: boolean
  }
  color?: 'primary' | 'green' | 'red' | 'yellow'
}

const colorClasses = {
  primary: 'bg-primary-100 text-primary-600 dark:bg-primary-900 dark:text-primary-400',
  green: 'bg-green-100 text-green-600 dark:bg-green-900 dark:text-green-400',
  red: 'bg-red-100 text-red-600 dark:bg-red-900 dark:text-red-400',
  yellow: 'bg-yellow-100 text-yellow-600 dark:bg-yellow-900 dark:text-yellow-400',
}

export default function StatCard({ title, value, icon, trend, color = 'primary' }: StatCardProps) {
  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow-md p-6">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm font-medium text-gray-600 dark:text-gray-400">{title}</p>
          <p className="text-3xl font-bold text-gray-900 dark:text-white mt-2">{value}</p>
          {trend && (
            <p
              className={`text-sm mt-2 ${
                trend.isPositive ? 'text-green-600' : 'text-red-600'
              }`}
            >
              {trend.isPositive ? '+' : ''}
              {trend.value}% from last week
            </p>
          )}
        </div>
        <div className={`p-4 rounded-lg ${colorClasses[color]}`}>{icon}</div>
      </div>
    </div>
  )
}
