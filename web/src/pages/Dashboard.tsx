import { useEffect, useState } from 'react'
import api from '../api/client'
import { Activity, Shield, Database, Clock, Users, Globe } from 'lucide-react'

interface Metrics {
  uptime_seconds: number
  cache_hits: number
  cache_misses: number
  cache_entries: number
  blacklist_domains: number
  whitelist_domains: number
  active_sessions: number
  log_pending: number
}

function StatCard({ icon: Icon, label, value, color }: {
  icon: React.ElementType
  label: string
  value: string | number
  color: string
}) {
  return (
    <div className="bg-white rounded-xl shadow-sm p-6 flex items-start gap-4">
      <div className={`p-3 rounded-lg ${color}`}>
        <Icon size={24} className="text-white" />
      </div>
      <div>
        <p className="text-sm text-gray-500">{label}</p>
        <p className="text-2xl font-bold text-gray-900">{value}</p>
      </div>
    </div>
  )
}

function formatUptime(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  return `${h}h ${m}m`
}

export default function Dashboard() {
  const [metrics, setMetrics] = useState<Metrics | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    const fetchMetrics = async () => {
      try {
        const res = await api.get('/metrics')
        setMetrics(res.data)
      } catch {
        setError('Erro ao carregar métricas')
      }
    }

    fetchMetrics()
    const interval = setInterval(fetchMetrics, 5000) // refresh every 5s
    return () => clearInterval(interval)
  }, [])

  if (error) {
    return <div className="text-red-600">{error}</div>
  }

  if (!metrics) {
    return <div className="text-gray-500">Carregando...</div>
  }

  const cacheTotal = metrics.cache_hits + metrics.cache_misses
  const hitRate = cacheTotal > 0
    ? ((metrics.cache_hits / cacheTotal) * 100).toFixed(1)
    : '0.0'

  return (
    <div>
      <h2 className="text-2xl font-bold text-gray-900 mb-6">Visão Geral</h2>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <StatCard
          icon={Clock}
          label="Uptime"
          value={formatUptime(metrics.uptime_seconds)}
          color="bg-blue-500"
        />
        <StatCard
          icon={Activity}
          label="Cache Hit Rate"
          value={`${hitRate}%`}
          color="bg-green-500"
        />
        <StatCard
          icon={Database}
          label="Entradas no Cache"
          value={metrics.cache_entries}
          color="bg-purple-500"
        />
        <StatCard
          icon={Shield}
          label="Domínios Bloqueados"
          value={metrics.blacklist_domains}
          color="bg-red-500"
        />
        <StatCard
          icon={Globe}
          label="Domínios Permitidos (whitelist)"
          value={metrics.whitelist_domains}
          color="bg-emerald-500"
        />
        <StatCard
          icon={Users}
          label="Sessões Ativas"
          value={metrics.active_sessions}
          color="bg-amber-500"
        />
      </div>
    </div>
  )
}