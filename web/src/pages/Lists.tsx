import { Fragment, useEffect, useState } from 'react'
import api from '../api/client'

interface BlocklistInfo {
  id: number
  name: string
  source_url: string
  list_type: number
  active: boolean
  domain_count: number
}

interface Category {
  id: number
  name: string
  description: string
}

const typeLabels: Record<number, { label: string; color: string }> = {
  0: { label: 'Blacklist', color: 'bg-red-100 text-red-700' },
  1: { label: 'Whitelist', color: 'bg-green-100 text-green-700' },
}

export default function Lists() {
  const [lists, setLists] = useState<BlocklistInfo[]>([])
  const [categories, setCategories] = useState<Category[]>([])
  const [name, setName] = useState('')
  const [sourceURL, setSourceURL] = useState('')
  const [listType, setListType] = useState(0)
  const [message, setMessage] = useState('')
  const [reloading, setReloading] = useState(false)
  const [downloading, setDownloading] = useState(false)
  const [editingCats, setEditingCats] = useState<number | null>(null)
  const [listCats, setListCats] = useState<number[]>([])
  const [savingCats, setSavingCats] = useState(false)
  const [addingDomains, setAddingDomains] = useState<number | null>(null)
  const [domainsText, setDomainsText] = useState('')
  const [savingDomains, setSavingDomains] = useState(false)

  const fetchLists = async () => {
    try {
      const res = await api.get('/lists')
      setLists(Array.isArray(res.data) ? res.data : [])
    } catch (err) {
      console.error('Erro ao carregar listas:', err)
    }
  }

  const fetchCategories = async () => {
    try {
      const res = await api.get('/categories')
      setCategories(Array.isArray(res.data) ? res.data : [])
    } catch (err) {
      console.error('Erro ao carregar categorias:', err)
    }
  }

  useEffect(() => {
    fetchLists()
    fetchCategories()
  }, [])

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

  const handleDownload = async () => {
    setDownloading(true)
    try {
      const res = await api.post('/lists/download')
      const data = res.data
      const details = Object.entries(data.downloaded || {})
        .map(([n, count]) => `${n}: ${count === -1 ? 'erro' : count + ' domínios'}`)
        .join(', ')
      setMessage(`Download concluído — ${details}. Total: ${data.blacklist_loaded} blacklist + ${data.whitelist_loaded} whitelist`)
      fetchLists()
      setTimeout(() => setMessage(''), 10000)
    } catch {
      setMessage('Erro ao baixar listas externas')
    } finally {
      setDownloading(false)
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

  const handleDelete = async (id: number) => {
    if (!confirm('Tem certeza? Os domínios da lista serão removidos.')) return
    try {
      await api.delete(`/lists/${id}`)
      setMessage('Lista removida')
      fetchLists()
      if (editingCats === id) setEditingCats(null)
      if (addingDomains === id) setAddingDomains(null)
      setTimeout(() => setMessage(''), 3000)
    } catch {
      setMessage('Erro ao remover lista')
    }
  }

  const openCategories = async (listId: number) => {
    if (editingCats === listId) {
      setEditingCats(null)
      return
    }
    setAddingDomains(null)
    try {
      const res = await api.get(`/lists/${listId}/categories`)
      const cats = res.data?.categories
      setListCats(Array.isArray(cats) ? cats : [])
      setEditingCats(listId)
    } catch {
      setMessage('Erro ao carregar categorias da lista')
    }
  }

  const toggleListCat = (catId: number) => {
    setListCats(prev =>
      prev.includes(catId) ? prev.filter(c => c !== catId) : [...prev, catId]
    )
  }

  const saveCategories = async () => {
    if (editingCats === null) return
    setSavingCats(true)
    try {
      await api.put(`/lists/${editingCats}/categories`, { categories: listCats })
      setMessage(`Categorias da lista atualizadas — ${listCats.length} categorias associadas`)
      setTimeout(() => setMessage(''), 3000)
    } catch {
      setMessage('Erro ao salvar categorias')
    } finally {
      setSavingCats(false)
    }
  }

  const openDomains = (listId: number) => {
    if (addingDomains === listId) {
      setAddingDomains(null)
      return
    }
    setEditingCats(null)
    setDomainsText('')
    setAddingDomains(listId)
  }

  const saveDomains = async () => {
    if (addingDomains === null) return
    const domains = domainsText
      .split('\n')
      .map(d => d.trim().toLowerCase())
      .filter(d => d.length > 0 && d.includes('.'))

    if (domains.length === 0) {
      setMessage('Nenhum domínio válido encontrado')
      return
    }

    setSavingDomains(true)
    try {
      const res = await api.post(`/lists/${addingDomains}/entries`, { domains })
      setMessage(`${res.data.inserted} domínios adicionados com sucesso`)
      setDomainsText('')
      setAddingDomains(null)
      fetchLists()
      setTimeout(() => setMessage(''), 3000)
    } catch {
      setMessage('Erro ao adicionar domínios')
    } finally {
      setSavingDomains(false)
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
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-2xl font-bold text-gray-900">Listas de Bloqueio</h2>
        <div className="flex gap-2">
          <button
            onClick={handleDownload}
            disabled={downloading}
            className="px-4 py-2 bg-green-600 text-white rounded-lg text-sm hover:bg-green-700 disabled:opacity-50 transition-colors"
          >
            {downloading ? 'Baixando...' : 'Baixar Listas Externas'}
          </button>
          <button
            onClick={handleReload}
            disabled={reloading}
            className="px-4 py-2 bg-amber-500 text-white rounded-lg text-sm hover:bg-amber-600 disabled:opacity-50 transition-colors"
          >
            {reloading ? 'Recarregando...' : 'Recarregar Listas'}
          </button>
        </div>
      </div>

      {message && (
        <div className={`px-4 py-2 rounded-lg text-sm mb-4 ${message.includes('Erro') ? 'bg-red-50 text-red-600' : 'bg-green-50 text-green-600'}`}>
          {message}
        </div>
      )}

      <div className="bg-white rounded-xl shadow-sm p-6 mb-6">
        <h3 className="text-lg font-semibold mb-4">Nova Lista</h3>
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
            <label className="block text-sm font-medium text-gray-700 mb-1">URL (opcional — deixe vazio para lista manual)</label>
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

      <div className="bg-white rounded-xl shadow-sm p-6 mb-6">
        <div className="text-sm text-gray-600 space-y-1">
          <p className="font-semibold text-gray-900">Como funciona:</p>
          <p>• Listas <strong>sem categoria</strong> associada → bloqueiam/permitem para <strong>toda a rede</strong> (global)</p>
          <p>• Listas <strong>com categoria</strong> associada → bloqueiam apenas para <strong>grupos que bloqueiam aquela categoria</strong> na política</p>
          <p>• Use o botão <strong>Domínios</strong> para adicionar domínios manualmente a qualquer lista</p>
          <p>• Após alterar listas ou domínios, clique <strong>Recarregar Listas</strong> para aplicar</p>
        </div>
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
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Ações</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {lists.map((l) => {
              const t = typeLabels[l.list_type] || { label: '?', color: 'bg-gray-100' }
              return (
                <Fragment key={l.id}>
                  <tr className="hover:bg-gray-50">
                    <td className="px-4 py-3 text-sm text-gray-500">{l.id}</td>
                    <td className="px-4 py-3 text-sm font-medium text-gray-900">{l.name}</td>
                    <td className="px-4 py-3">
                      <span className={`px-2 py-1 rounded text-xs font-medium ${t.color}`}>{t.label}</span>
                    </td>
                    <td className="px-4 py-3 text-sm font-mono text-gray-700">{l.domain_count.toLocaleString()}</td>
                    <td className="px-4 py-3">
                      <span className={`px-2 py-1 rounded text-xs font-medium ${l.active ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'}`}>
                        {l.active ? 'Ativa' : 'Inativa'}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-sm text-gray-500 max-w-xs truncate">
                      {l.source_url || 'Manual'}
                    </td>
                    <td className="px-4 py-3 text-sm">
                      <div className="flex gap-2 flex-wrap">
                      <button
                        onClick={() => openDomains(l.id)}
                        className={`px-3 py-1 rounded text-xs font-medium transition-colors ${
                          addingDomains === l.id
                            ? 'bg-green-100 text-green-700'
                            : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
                        }`}
                      >
                        {addingDomains === l.id ? 'Fechar' : 'Domínios'}
                      </button>
                      <button
                        onClick={() => openCategories(l.id)}
                        className={`px-3 py-1 rounded text-xs font-medium transition-colors ${
                          editingCats === l.id
                            ? 'bg-purple-100 text-purple-700'
                            : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
                        }`}
                      >
                        {editingCats === l.id ? 'Fechar' : 'Categorias'}
                      </button>
                      <button
                        onClick={() => handleDelete(l.id)}
                        className="px-3 py-1 bg-red-50 text-red-600 rounded text-xs font-medium hover:bg-red-100 transition-colors"
                      >
                        Remover
                      </button>
                    </div>
                    </td>
                  </tr>

                  {addingDomains === l.id && (
                    <tr>
                      <td colSpan={7} className="px-4 py-4 bg-green-50">
                        <div className="mb-3">
                          <h4 className="text-sm font-semibold text-gray-900 mb-1">
                            Adicionar domínios à lista &quot;{l.name}&quot;
                          </h4>
                          <p className="text-xs text-gray-500 mb-3">
                            Digite um domínio por linha. Exemplos: facebook.com, instagram.com, tiktok.com
                          </p>
                        </div>
                        <textarea
                          value={domainsText}
                          onChange={(e) => setDomainsText(e.target.value)}
                          rows={6}
                          className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm font-mono focus:outline-none focus:ring-2 focus:ring-green-500 mb-3"
                          placeholder={"facebook.com\ninstagram.com\ntiktok.com\ntwitter.com\nx.com"}
                        />
                        <div className="flex items-center gap-3">
                          <button
                            onClick={saveDomains}
                            disabled={savingDomains || !domainsText.trim()}
                            className="px-4 py-2 bg-green-600 text-white rounded-lg text-sm hover:bg-green-700 disabled:opacity-50 transition-colors"
                          >
                            {savingDomains ? 'Salvando...' : 'Adicionar Domínios'}
                          </button>
                          <span className="text-xs text-gray-500">
                            {domainsText.split('\n').filter(d => d.trim().length > 0 && d.includes('.')).length} domínios válidos
                          </span>
                        </div>
                      </td>
                    </tr>
                  )}

                  {editingCats === l.id && (
                    <tr>
                      <td colSpan={7} className="px-4 py-4 bg-purple-50">
                        <div className="mb-3">
                          <h4 className="text-sm font-semibold text-gray-900 mb-1">
                            Categorias da lista &quot;{l.name}&quot;
                          </h4>
                          <p className="text-xs text-gray-500 mb-3">
                            Selecione a quais categorias esta lista pertence. Se associada a categorias, os domínios só serão bloqueados para grupos que bloqueiam essas categorias.
                            Se nenhuma categoria for selecionada, a lista bloqueia para toda a rede (global).
                          </p>
                        </div>
                        <div className="grid grid-cols-2 md:grid-cols-3 gap-2 mb-4">
                          {categories.map((cat) => (
                            <label
                              key={cat.id}
                              className={`flex items-center gap-2 p-3 rounded-lg cursor-pointer transition-colors ${
                                listCats.includes(cat.id)
                                  ? 'bg-purple-100 border border-purple-300'
                                  : 'bg-white border border-gray-200 hover:border-gray-300'
                              }`}
                            >
                              <input
                                type="checkbox"
                                checked={listCats.includes(cat.id)}
                                onChange={() => toggleListCat(cat.id)}
                                className="rounded text-purple-600 focus:ring-purple-500"
                              />
                              <div>
                                <span className="text-sm font-medium text-gray-900">
                                  {catLabels[cat.name] || cat.name}
                                </span>
                              </div>
                            </label>
                          ))}
                        </div>
                        {categories.length === 0 && (
                          <p className="text-sm text-gray-400 mb-4">Nenhuma categoria cadastrada no banco.</p>
                        )}
                        <button
                          onClick={saveCategories}
                          disabled={savingCats}
                          className="px-4 py-2 bg-purple-600 text-white rounded-lg text-sm hover:bg-purple-700 disabled:opacity-50 transition-colors"
                        >
                          {savingCats ? 'Salvando...' : 'Salvar Categorias'}
                        </button>
                      </td>
                    </tr>
                  )}
                </Fragment>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}