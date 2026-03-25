import { useEffect, useState } from 'react'
import api from '../api/client'

interface BlocklistInfo {
  id: number
  name: string
  source_url: string
  list_type: number
  active: boolean
  domain_count: number
}

const typeLabels: Record<number, { label: string; color: string }> = {
  0: { label: 'Blacklist', color: 'bg-red-100 text-red-700' },
  1: { label: 'Whitelist', color: 'bg-green-100 text-green-700' },
}

export default function Lists() {
  const [lists, setLists] = useState<BlocklistInfo[]>([])
  const [name, setName] = useState('')
  const [sourceURL, setSourceURL] = useState('')
  const [listType, setListType] = useState(0)
  const [message, setMessage] = useState('')
  const [reloading, setReloading] = useState(false)

  const fetchLists = async () => {
    try {
      const res = await api.get('/lists')
      setLists(res.data || [])
    } catch (err) {
      console.error('Erro ao carregar listas:', err)
    }
  }

  useEffect(() => { fetchLists() }, [])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await api.post('/lists', { name, source_url: sourceURL, list_type: listType })
      setName('')
      setSourceURL('')
      setListType(0)
      setMessage('Lista criada com sucesso')
      fetchLists()
      setTimeout(() => setMessage(''), 3000)
    } catch {
      setMessage('Erro ao criar lista')
    }
  }

  const handleReload = async () => {
    setReloading(true)
    try {
      const res = await api.post('/lists/reload')
      setMessage(`Recarregado: ${res.data.blacklist} blacklist + ${res.data.whitelist} whitelist`)
      setTimeout(() => setMessage(''), 3000)
    } catch {
      setMessage('Erro ao recarregar listas')
    } finally {
      setReloading(false)
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-2xl font-bold text-gray-900">Listas de Bloqueio</h2>
        <button
          onClick={handleReload}
          disabled={reloading}
          className="px-4 py-2 bg-amber-500 text-white rounded-lg text-sm hover:bg-amber-600 disabled:opacity-50 transition-colors"
        >
          {reloading ? 'Recarregando...' : 'Recarregar Listas'}
        </button>
      </div>

      <div className="bg-white rounded-xl shadow-sm p-6 mb-6">
        <h3 className="text-lg font-semibold mb-4">Nova Lista</h3>
        {message && (
          <div className={`px-4 py-2 rounded-lg text-sm mb-4 ${message.includes('Erro') ? 'bg-red-50 text-red-600' : 'bg-green-50 text-green-600'}`}>
            {message}
          </div>
        )}
        <form onSubmit={handleCreate} className="flex gap-4 flex-wrap items-end">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Nome</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              required
            />
          </div>
          <div className="flex-1">
            <label className="block text-sm font-medium text-gray-700 mb-1">URL (opcional para listas externas)</label>
            <input
              type="text"
              value={sourceURL}
              onChange={(e) => setSourceURL(e.target.value)}
              placeholder="https://raw.githubusercontent.com/..."
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Tipo</label>
            <select
              value={listType}
              onChange={(e) => setListType(Number(e.target.value))}
              className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option value={0}>Blacklist</option>
              <option value={1}>Whitelist</option>
            </select>
          </div>
          <button type="submit" className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700 transition-colors">
            Criar
          </button>
        </form>
      </div>

      <div className="bg-white rounded-xl shadow-sm overflow-hidden">
        <table className="w-full">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">ID</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Nome</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Tipo</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Domínios</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Fonte</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {lists.map((l) => {
              const t = typeLabels[l.list_type] || { label: '?', color: 'bg-gray-100' }
              return (
                <tr key={l.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3 text-sm text-gray-700">{l.id}</td>
                  <td className="px-4 py-3 text-sm text-gray-700 font-medium">{l.name}</td>
                  <td className="px-4 py-3 text-sm">
                    <span className={`px-2 py-1 rounded-full text-xs font-medium ${t.color}`}>{t.label}</span>
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-700">{l.domain_count}</td>
                  <td className="px-4 py-3 text-sm">
                    <span className={`px-2 py-1 rounded-full text-xs font-medium ${l.active ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'}`}>
                      {l.active ? 'Ativa' : 'Inativa'}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-500 truncate max-w-xs">{l.source_url || 'Manual'}</td>
                </tr>
              )
            })}
            {lists.length === 0 && (
              <tr><td colSpan={6} className="px-4 py-8 text-center text-gray-400">Nenhuma lista cadastrada</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}