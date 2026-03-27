import { useEffect, useState } from 'react'
import api from '../api/client'
import { Activity, Shield, Database, Clock, Users, Globe, TrendingUp, Ban } from 'lucide-react'

interface Metrics {
  uptime_seconds: number
  cache_hits: number
  cache_misses: number
  cache_entries: number
  blacklist_domains: number
  whitelist_domains: number
  active_sessions: number
  log_pending: number
  total_queries: number
  queries_per_second: number
  avg_latency_ms: number
}

interface TopItem {
  name: string
  count: number
}

interface DashboardStats {
  total_today: number
  total_week: number
  total_month: number
  blocked_percent: number
  top_domains: TopItem[] | null
  top_blocked: TopItem[] | null
  top_clients: TopItem[] | null
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

function TopList({ title, items, icon: Icon }: {
  title: string
  items: TopItem[] | null
  icon: React.ElementType
}) {
  return (
    <div className="bg-white rounded-xl shadow-sm p-6">
      <div className="flex items-center gap-2 mb-4">
        <Icon size={18} className="text-gray-400" />
        <h3 className="font-semibold text-gray-900">{title}</h3>
      </div>
      {(!items || items.length === 0) ? (
        <p className="text-sm text-gray-400">Sem dados ainda</p>
      ) : (
        <div className="space-y-2">
          {items.map((item, i) => (
            <div key={i} className="flex items-center justify-between text-sm">
              <span className="text-gray-700 truncate flex-1 mr-2">{item.name}</span>
              <span className="text-gray-500 font-mono">{item.count}</span>
            </div>
          ))}
        </div>
      )}
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
  const [stats, setStats] = useState<DashboardStats | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [metricsRes, statsRes] = await Promise.all([
          api.get('/metrics'),
          api.get('/dashboard'),
        ])
        setMetrics(metricsRes.data)
        setStats(statsRes.data)
      } catch {
        setError('Erro ao carregar dados')
      }
    }

    fetchData()
    const interval = setInterval(fetchData, 5000)
    return () => clearInterval(interval)
  }, [])

  if (error) return <div className="text-red-600">{error}</div>
  if (!metrics) return <div className="text-gray-500">Carregando...</div>

  const cacheTotal = metrics.cache_hits + metrics.cache_misses
  const hitRate = cacheTotal > 0
    ? ((metrics.cache_hits / cacheTotal) * 100).toFixed(1)
    : '0.0'

  return (
    <div>
      <h2 className="text-2xl font-bold text-gray-900 mb-6">Visão Geral</h2>

      {/* Stat cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <StatCard icon={Clock} label="Uptime" value={formatUptime(metrics.uptime_seconds)} color="bg-blue-500" />
        <StatCard icon={TrendingUp} label="Queries/s" value={metrics.queries_per_second?.toFixed(1) || '0'} color="bg-indigo-500" />
        <StatCard icon={Activity} label="Cache Hit Rate" value={`${hitRate}%`} color="bg-green-500" />
        <StatCard icon={Database} label="Latência Média" value={`${metrics.avg_latency_ms?.toFixed(2) || '0'}ms`} color="bg-purple-500" />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <StatCard icon={Globe} label="Queries Hoje" value={stats?.total_today || 0} color="bg-cyan-500" />
        <StatCard icon={Shield} label="% Bloqueado" value={`${stats?.blocked_percent?.toFixed(1) || '0'}%`} color="bg-red-500" />
        <StatCard icon={Ban} label="Domínios Bloqueados" value={metrics.blacklist_domains} color="bg-orange-500" />
        <StatCard icon={Users} label="Sessões Ativas" value={metrics.active_sessions} color="bg-amber-500" />
      </div>

      {/* Top lists */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <TopList title="Top Domínios Consultados" items={stats?.top_domains || null} icon={Globe} />
        <TopList title="Top Domínios Bloqueados" items={stats?.top_blocked || null} icon={Ban} />
        <TopList title="Top Clientes por Volume" items={stats?.top_clients || null} icon={Users} />
      </div>
    </div>
  )
}