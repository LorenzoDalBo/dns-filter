import { useEffect, useState } from 'react'
import {
  useReactTable,
  getCoreRowModel,
  flexRender,
  createColumnHelper,
} from '@tanstack/react-table'
import api from '../api/client'

interface LogEntry {
  queried_at: string
  client_ip: string
  domain: string
  query_type: number
  action: number
  response_ms: number
  upstream: string
}

const queryTypes: Record<number, string> = {
  1: 'A', 5: 'CNAME', 15: 'MX', 28: 'AAAA', 2: 'NS', 16: 'TXT',
}

const actionLabels: Record<number, { label: string; color: string }> = {
  0: { label: 'Permitido', color: 'bg-green-100 text-green-700' },
  1: { label: 'Bloqueado', color: 'bg-red-100 text-red-700' },
  2: { label: 'Cache', color: 'bg-blue-100 text-blue-700' },
}

const col = createColumnHelper<LogEntry>()

const columns = [
  col.accessor('queried_at', {
    header: 'Data/Hora',
    cell: (info) => new Date(info.getValue()).toLocaleString('pt-BR'),
  }),
  col.accessor('client_ip', {
    header: 'IP Cliente',
    cell: (info) => info.getValue()?.replace('/32', '') || '-',
  }),
  col.accessor('domain', {
    header: 'Domínio',
  }),
  col.accessor('query_type', {
    header: 'Tipo',
    cell: (info) => queryTypes[info.getValue()] || info.getValue(),
  }),
  col.accessor('action', {
    header: 'Ação',
    cell: (info) => {
      const a = actionLabels[info.getValue()] || { label: '?', color: 'bg-gray-100' }
      return (
        <span className={`px-2 py-1 rounded-full text-xs font-medium ${a.color}`}>
          {a.label}
        </span>
      )
    },
  }),
  col.accessor('response_ms', {
    header: 'Latência',
    cell: (info) => `${info.getValue()?.toFixed(1) || 0}ms`,
  }),
  col.accessor('upstream', {
    header: 'Upstream',
  }),
]

export default function Logs() {
  const [data, setData] = useState<LogEntry[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [domain, setDomain] = useState('')
  const [action, setAction] = useState('')
  const [clientIP, setClientIP] = useState('')
  const limit = 20

  const fetchLogs = async () => {
    try {
      const params = new URLSearchParams()
      params.set('limit', String(limit))
      params.set('offset', String(offset))
      if (domain) params.set('domain', domain)
      if (action) params.set('action', action)
      if (clientIP) params.set('client_ip', clientIP)

      const res = await api.get(`/logs?${params}`)
      setData(res.data.data || [])
      setTotal(res.data.total || 0)
    } catch (err) {
      console.error('Erro ao carregar logs:', err)
    }
  }

  useEffect(() => {
    fetchLogs()
  }, [offset, domain, action, clientIP])

  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
  })

  const totalPages = Math.ceil(total / limit)
  const currentPage = Math.floor(offset / limit) + 1

  return (
    <div>
      <h2 className="text-2xl font-bold text-gray-900 mb-6">Logs DNS</h2>

      {/* Filters */}
      <div className="bg-white rounded-xl shadow-sm p-4 mb-4 flex gap-4 flex-wrap">
        <input
          type="text"
          placeholder="Buscar domínio..."
          value={domain}
          onChange={(e) => { setDomain(e.target.value); setOffset(0) }}
          className="px-3 py-2 border border-gray-300 rounded-lg text-sm flex-1 min-w-48 focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
        <input
          type="text"
          placeholder="IP do cliente..."
          value={clientIP}
          onChange={(e) => { setClientIP(e.target.value); setOffset(0) }}
          className="px-3 py-2 border border-gray-300 rounded-lg text-sm w-40 focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
        <select
          value={action}
          onChange={(e) => { setAction(e.target.value); setOffset(0) }}
          className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        >
          <option value="">Todas as ações</option>
          <option value="0">Permitido</option>
          <option value="1">Bloqueado</option>
          <option value="2">Cache</option>
        </select>
        <button
          onClick={fetchLogs}
          className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700 transition-colors"
        >
          Buscar
        </button>
      </div>

      {/* Table */}
      <div className="bg-white rounded-xl shadow-sm overflow-hidden">
        <table className="w-full">
          <thead className="bg-gray-50">
            {table.getHeaderGroups().map((hg) => (
              <tr key={hg.id}>
                {hg.headers.map((header) => (
                  <th key={header.id} className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                    {flexRender(header.column.columnDef.header, header.getContext())}
                  </th>
                ))}
              </tr>
            ))}
          </thead>
          <tbody className="divide-y divide-gray-100">
            {table.getRowModel().rows.map((row) => (
              <tr key={row.id} className="hover:bg-gray-50">
                {row.getVisibleCells().map((cell) => (
                  <td key={cell.id} className="px-4 py-3 text-sm text-gray-700">
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </td>
                ))}
              </tr>
            ))}
            {data.length === 0 && (
              <tr>
                <td colSpan={columns.length} className="px-4 py-8 text-center text-gray-400">
                  Nenhum log encontrado
                </td>
              </tr>
            )}
          </tbody>
        </table>

        {/* Pagination */}
        <div className="px-4 py-3 bg-gray-50 flex items-center justify-between text-sm">
          <span className="text-gray-500">{total} registros</span>
          <div className="flex gap-2">
            <button
              disabled={offset === 0}
              onClick={() => setOffset(Math.max(0, offset - limit))}
              className="px-3 py-1 border rounded-lg disabled:opacity-50 hover:bg-gray-100"
            >
              Anterior
            </button>
            <span className="px-3 py-1 text-gray-600">
              Página {currentPage} de {totalPages || 1}
            </span>
            <button
              disabled={offset + limit >= total}
              onClick={() => setOffset(offset + limit)}
              className="px-3 py-1 border rounded-lg disabled:opacity-50 hover:bg-gray-100"
            >
              Próxima
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}