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

let dynamicToken: string | null = null;

/**
 * Sets the authentication token to be used for all subsequent API requests.
 * This is used to dynamically update the token when it's refreshed by Auth0.
 */
export const setAuthToken = (token: string | null) => {
  dynamicToken = token;
};

// Request interceptor for auth
apiClient.interceptors.request.use(
  (config) => {
    // Priority: 1. Dynamically set token, 2. localStorage, 3. Environment variable
    const token = dynamicToken || (typeof window !== 'undefined' ? localStorage.getItem('auth_token') : null) || API_TOKEN;
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// Response interceptor for errors
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    // Log detailed error information for debugging
    if (error.response) {
      // Server responded with error status
      console.error('API Error Response:', {
        status: error.response.status,
        statusText: error.response.statusText,
        method: error.config?.method?.toUpperCase(),
        url: error.config?.url,
        data: error.response.data,
        headers: error.response.headers,
      });

      // Log the actual error message from the API if available
      if (error.response.data?.message) {
        console.error('API Error Message:', error.response.data.message);
      }
      if (error.response.data?.error) {
        console.error('API Error Details:', error.response.data.error);
      }
    } else if (error.request) {
      // Request was made but no response received
      console.error('API No Response:', {
        method: error.config?.method?.toUpperCase(),
        url: error.config?.url,
        message: 'No response received from server',
      });
    } else {
      // Something else happened
      console.error('API Request Error:', error.message);
    }

    if (error.response?.status === 401) {
      // Unauthorized - handled by ProtectedRoute in the UI
      console.warn('Unauthorized request detected');
    }
    return Promise.reject(error);
  }
);
