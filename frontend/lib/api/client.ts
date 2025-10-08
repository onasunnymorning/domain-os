import axios from 'axios';

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
const API_TOKEN = process.env.NEXT_PUBLIC_API_TOKEN || '';

// Debug logging
console.log('API Client Configuration:', {
  baseURL: API_BASE_URL,
  hasToken: !!API_TOKEN,
  tokenLength: API_TOKEN?.length || 0
});

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${API_TOKEN}`,
  },
});

// Request interceptor for auth
apiClient.interceptors.request.use(
  (config) => {
    // Get token from localStorage if available (for dynamic auth later)
    if (typeof window !== 'undefined') {
      const token = localStorage.getItem('auth_token') || API_TOKEN;
      if (token) {
        config.headers.Authorization = `Bearer ${token}`;
      }
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// Response interceptor for errors
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      // Redirect to login (will implement later)
      if (typeof window !== 'undefined') {
        window.location.href = '/login';
      }
    }
    return Promise.reject(error);
  }
);
