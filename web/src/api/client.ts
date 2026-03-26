import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
})

// Inject JWT token into every request
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// Handle 401 and auto-refresh token
let isRefreshing = false

api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config

    if (error.response?.status === 401 && !originalRequest._retry) {
      // If we're already refreshing, redirect to login
      if (isRefreshing) {
        localStorage.removeItem('token')
        window.location.href = '/login'
        return Promise.reject(error)
      }

      originalRequest._retry = true
      isRefreshing = true

      try {
        const token = localStorage.getItem('token')
        if (!token) throw new Error('No token')

        const res = await axios.post('/api/auth/refresh', {}, {
          headers: { Authorization: `Bearer ${token}` }
        })

        const newToken = res.data.token
        localStorage.setItem('token', newToken)
        originalRequest.headers.Authorization = `Bearer ${newToken}`
        isRefreshing = false
        return api(originalRequest)
      } catch {
        isRefreshing = false
        localStorage.removeItem('token')
        window.location.href = '/login'
        return Promise.reject(error)
      }
    }

    return Promise.reject(error)
  }
)

export default api