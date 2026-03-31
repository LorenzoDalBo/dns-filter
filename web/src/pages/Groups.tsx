import { useEffect, useState, Fragment } from 'react'
import api from '../api/client'

interface Group {
  id: number
  name: string
  description: string
}

interface Category {
  id: number
  name: string
  description: string
}

export default function Groups() {
  const [groups, setGroups] = useState<Group[]>([])
  const [categories, setCategories] = useState<Category[]>([])
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [message, setMessage] = useState('')
  const [editingPolicy, setEditingPolicy] = useState<number | null>(null)
  const [blockedCats, setBlockedCats] = useState<number[]>([])
  const [savingPolicy, setSavingPolicy] = useState(false)

  const fetchGroups = async () => {
    try {
      const res = await api.get('/groups')
      setGroups(res.data || [])
    } catch (err) {
      console.error('Erro ao carregar grupos:', err)
    }
  }

  const fetchCategories = async () => {
    try {
      const res = await api.get('/categories')
      setCategories(res.data || [])
    } catch (err) {
      console.error('Erro ao carregar categorias:', err)
    }
  }

  useEffect(() => {
    fetchGroups()
    fetchCategories()
  }, [])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await api.post('/groups', { name, description })
      setName('')
      setDescription('')
      setMessage('Grupo criado com sucesso')
      fetchGroups()
      setTimeout(() => setMessage(''), 3000)
    } catch {
      setMessage('Erro ao criar grupo')
    }
  }

  const handleDelete = async (id: number) => {
    if (!confirm('Tem certeza que deseja remover este grupo?')) return
    try {
      await api.delete(`/groups/${id}`)
      setMessage('Grupo removido')
      fetchGroups()
      if (editingPolicy === id) setEditingPolicy(null)
      setTimeout(() => setMessage(''), 3000)
    } catch {
      setMessage('Erro ao remover grupo')
    }
  }

  const openPolicy = async (groupId: number) => {
    if (editingPolicy === groupId) {
      setEditingPolicy(null)
      return
    }
    try {
      const res = await api.get(`/groups/${groupId}/policy`)
      setBlockedCats(res.data.blocked_categories || [])
      setEditingPolicy(groupId)
    } catch {
      setMessage('Erro ao carregar política')
    }
  }

  const toggleCategory = (catId: number) => {
    setBlockedCats(prev =>
      prev.includes(catId) ? prev.filter(c => c !== catId) : [...prev, catId]
    )
  }

  const savePolicy = async () => {
    if (editingPolicy === null) return
    setSavingPolicy(true)
    try {
      await api.put(`/groups/${editingPolicy}/policy`, { categories: blockedCats })
      setMessage(`Política do grupo atualizada — ${blockedCats.length} categorias bloqueadas`)
      setTimeout(() => setMessage(''), 3000)
    } catch {
      setMessage('Erro ao salvar política')
    } finally {
      setSavingPolicy(false)
    }
  }

  const catLabels: Record<string, string> = {
    malware: 'Malware e Phishing',
    ads: 'Publicidade e Rastreamento',
    adult: 'Conteúdo Adulto',
    social: 'Redes Sociais',
    streaming: 'Streaming de Vídeo',
    gaming: 'Jogos Online',
  }

  return (
    <div>
      <h2 className="text-2xl font-bold text-gray-900 mb-6">Grupos e Políticas</h2>

      {message && (
        <div className={`px-4 py-2 rounded-lg text-sm mb-4 ${message.includes('Erro') ? 'bg-red-50 text-red-600' : 'bg-green-50 text-green-600'}`}>
          {message}
        </div>
      )}

      <div className="bg-white rounded-xl shadow-sm p-6 mb-6">
        <h3 className="text-lg font-semibold mb-4">Novo Grupo</h3>
        <form onSubmit={handleCreate} className="flex gap-4 flex-wrap items-end">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Nome</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="Ex: Funcionários"
              required
            />
          </div>
          <div className="flex-1">
            <label className="block text-sm font-medium text-gray-700 mb-1">Descrição</label>
            <input
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="Descrição do grupo e sua política"
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
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Nome</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Descrição</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Ações</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
             {groups.map((g) => (
              <Fragment key={g.id}>
                <tr className="hover:bg-gray-50">
                  <td className="px-4 py-3 text-sm text-gray-500">{g.id}</td>
                  <td className="px-4 py-3 text-sm font-medium text-gray-900">{g.name}</td>
                  <td className="px-4 py-3 text-sm text-gray-500">{g.description}</td>
                  <td className="px-4 py-3 text-sm flex gap-2">
                    <button
                      onClick={() => openPolicy(g.id)}
                      className={`px-3 py-1 rounded text-xs font-medium transition-colors ${
                        editingPolicy === g.id
                          ? 'bg-blue-100 text-blue-700'
                          : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
                      }`}
                    >
                      {editingPolicy === g.id ? 'Fechar Política' : 'Editar Política'}
                    </button>
                    <button
                      onClick={() => handleDelete(g.id)}
                      className="px-3 py-1 bg-red-50 text-red-600 rounded text-xs font-medium hover:bg-red-100 transition-colors"
                    >
                      Remover
                    </button>
                  </td>
                </tr>
                {editingPolicy === g.id && (
                  <tr key={`policy-${g.id}`}>
                    <td colSpan={4} className="px-4 py-4 bg-blue-50">
                      <div className="mb-3">
                        <h4 className="text-sm font-semibold text-gray-900 mb-1">
                          Categorias bloqueadas para o grupo "{g.name}"
                        </h4>
                        <p className="text-xs text-gray-500 mb-3">
                          Marque as categorias que este grupo NÃO pode acessar. Listas associadas a essas categorias serão bloqueadas.
                        </p>
                      </div>
                      <div className="grid grid-cols-2 md:grid-cols-3 gap-2 mb-4">
                        {categories.map((cat) => (
                          <label
                            key={cat.id}
                            className={`flex items-center gap-2 p-3 rounded-lg cursor-pointer transition-colors ${
                              blockedCats.includes(cat.id)
                                ? 'bg-red-100 border border-red-300'
                                : 'bg-white border border-gray-200 hover:border-gray-300'
                            }`}
                          >
                            <input
                              type="checkbox"
                              checked={blockedCats.includes(cat.id)}
                              onChange={() => toggleCategory(cat.id)}
                              className="rounded text-red-600 focus:ring-red-500"
                            />
                            <div>
                              <span className="text-sm font-medium text-gray-900">
                                {catLabels[cat.name] || cat.name}
                              </span>
                              {cat.description && (
                                <p className="text-xs text-gray-500">{cat.description}</p>
                              )}
                            </div>
                          </label>
                        ))}
                      </div>
                      {categories.length === 0 && (
                        <p className="text-sm text-gray-400 mb-4">Nenhuma categoria cadastrada no banco.</p>
                      )}
                      <button
                        onClick={savePolicy}
                        disabled={savingPolicy}
                        className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700 disabled:opacity-50 transition-colors"
                      >
                        {savingPolicy ? 'Salvando...' : 'Salvar Política'}
                      </button>
                    </td>
                  </tr>
                )}
              </Fragment>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}