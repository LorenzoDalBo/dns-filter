import { useEffect, useState } from 'react'
import api from '../api/client'

interface IPRange {
  id: number
  cidr: string
  group_id: number
  auth_mode: number
  description: string
}

interface Group {
  id: number
  name: string
}

const authModeLabels: Record<number, { label: string; color: string }> = {
  0: { label: 'Sem autenticação', color: 'bg-gray-100 text-gray-600' },
  1: { label: 'Captive Portal', color: 'bg-amber-100 text-amber-700' },
}

export default function Ranges() {
  const [ranges, setRanges] = useState<IPRange[]>([])
  const [groups, setGroups] = useState<Group[]>([])
  const [cidr, setCidr] = useState('')
  const [groupId, setGroupId] = useState(1)
  const [authMode, setAuthMode] = useState(0)
  const [description, setDescription] = useState('')
  const [message, setMessage] = useState('')

  const fetchRanges = async () => {
    try {
      const res = await api.get('/ranges')
      setRanges(res.data || [])
    } catch (err) {
      console.error('Erro ao carregar ranges:', err)
    }
  }

  const fetchGroups = async () => {
    try {
      const res = await api.get('/groups')
      setGroups(res.data || [])
    } catch (err) {
      console.error('Erro ao carregar grupos:', err)
    }
  }

  useEffect(() => {
    fetchRanges()
    fetchGroups()
  }, [])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await api.post('/ranges', { cidr, group_id: groupId, auth_mode: authMode, description })
      setCidr('')
      setDescription('')
      setMessage('Range criado com sucesso')
      fetchRanges()
      setTimeout(() => setMessage(''), 3000)
    } catch {
      setMessage('Erro ao criar range — verifique o formato CIDR')
    }
  }

  const groupName = (id: number) => groups.find(g => g.id === id)?.name || `Grupo ${id}`

  return (
    <div>
      <h2 className="text-2xl font-bold text-gray-900 mb-6">Faixas de IP</h2>

      <div className="bg-white rounded-xl shadow-sm p-6 mb-6">
        <h3 className="text-lg font-semibold mb-4">Nova Faixa</h3>
        {message && (
          <div className={`px-4 py-2 rounded-lg text-sm mb-4 ${message.includes('Erro') ? 'bg-red-50 text-red-600' : 'bg-green-50 text-green-600'}`}>
            {message}
          </div>
        )}
        <form onSubmit={handleCreate} className="flex gap-4 flex-wrap items-end">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">CIDR</label>
            <input
              type="text"
              value={cidr}
              onChange={(e) => setCidr(e.target.value)}
              placeholder="192.168.1.0/24"
              className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              required
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Grupo</label>
            <select
              value={groupId}
              onChange={(e) => setGroupId(Number(e.target.value))}
              className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              {groups.map(g => (
                <option key={g.id} value={g.id}>{g.name}</option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Autenticação</label>
            <select
              value={authMode}
              onChange={(e) => setAuthMode(Number(e.target.value))}
              className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option value={0}>Sem autenticação</option>
              <option value={1}>Captive Portal</option>
            </select>
          </div>
          <div className="flex-1">
            <label className="block text-sm font-medium text-gray-700 mb-1">Descrição</label>
            <input
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Ex: VLAN de visitantes"
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
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
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">CIDR</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Grupo</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Autenticação</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Descrição</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {ranges.map((r) => {
              const a = authModeLabels[r.auth_mode] || { label: '?', color: 'bg-gray-100' }
              return (
                <tr key={r.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3 text-sm text-gray-700">{r.id}</td>
                  <td className="px-4 py-3 text-sm text-gray-700 font-mono">{r.cidr}</td>
                  <td className="px-4 py-3 text-sm text-gray-700">{groupName(r.group_id)}</td>
                  <td className="px-4 py-3 text-sm">
                    <span className={`px-2 py-1 rounded-full text-xs font-medium ${a.color}`}>{a.label}</span>
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-500">{r.description || '-'}</td>
                </tr>
              )
            })}
            {ranges.length === 0 && (
              <tr><td colSpan={5} className="px-4 py-8 text-center text-gray-400">Nenhuma faixa cadastrada</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}