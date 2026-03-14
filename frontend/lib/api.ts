import axios, { AxiosInstance } from "axios";
import { clearAccessToken, getAccessToken, setAccessToken } from "./auth";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

const api: AxiosInstance = axios.create({
  baseURL: API_URL,
  withCredentials: true, // sends HttpOnly cookie for refresh token
  timeout: 30_000,
});

// Attach access token on every request
api.interceptors.request.use((config) => {
  const token = getAccessToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Auto-refresh on 401
let isRefreshing = false;
let failedQueue: Array<{
  resolve: (value: unknown) => void;
  reject: (reason?: unknown) => void;
}> = [];

function processQueue(error: unknown, token: string | null = null) {
  failedQueue.forEach(({ resolve, reject }) => {
    if (error) {
      reject(error);
    } else {
      resolve(token);
    }
  });
  failedQueue = [];
}

api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const original = error.config;

    if (error.response?.status === 401 && !original._retry) {
      if (isRefreshing) {
        return new Promise((resolve, reject) => {
          failedQueue.push({ resolve, reject });
        })
          .then((token) => {
            original.headers.Authorization = `Bearer ${token}`;
            return api(original);
          })
          .catch((err) => Promise.reject(err));
      }

      original._retry = true;
      isRefreshing = true;

      try {
        const { data } = await axios.post(
          `${API_URL}/api/auth/refresh`,
          {},
          { withCredentials: true }
        );
        setAccessToken(data.access_token);
        processQueue(null, data.access_token);
        original.headers.Authorization = `Bearer ${data.access_token}`;
        return api(original);
      } catch (refreshError) {
        processQueue(refreshError, null);
        clearAccessToken();
        if (typeof window !== "undefined") {
          window.location.href = "/login";
        }
        return Promise.reject(refreshError);
      } finally {
        isRefreshing = false;
      }
    }

    return Promise.reject(error);
  }
);

export default api;

// ─── Auth API ──────────────────────────────────────────────────────────────────
export const authApi = {
  register: (data: { email: string; username: string; password: string }) =>
    api.post("/api/auth/register", data),

  login: (data: { email: string; password: string }) =>
    api.post("/api/auth/login", data),

  logout: () => api.post("/api/auth/logout"),

  refresh: () => api.post("/api/auth/refresh"),

  me: () => api.get("/api/auth/me"),
};

// ─── Upload API ────────────────────────────────────────────────────────────────
export const uploadApi = {
  sign: (data: { filename: string; content_type: string; size_bytes: number }) =>
    api.post("/api/upload/sign", data),

  confirm: (mediaId: string) =>
    api.post("/api/upload/confirm", { media_id: mediaId }),
};

// ─── Media API ─────────────────────────────────────────────────────────────────
export const mediaApi = {
  list: (params?: { page?: number; page_size?: number }) =>
    api.get("/api/media", { params }),

  get: (id: string) => api.get(`/api/media/${id}`),

  update: (id: string, data: { title?: string; description?: string; tags?: string[] }) =>
    api.patch(`/api/media/${id}`, data),

  delete: (id: string) => api.delete(`/api/media/${id}`),
};

// ─── Public API ────────────────────────────────────────────────────────────────
export const publicApi = {
  explore: (params?: { cursor?: string; page_size?: number; type?: string }) =>
    api.get("/api/public/explore", { params }),

  search: (params: { q: string; page?: number; page_size?: number }) =>
    api.get("/api/public/search", { params }),

  getByShortCode: (shortCode: string) =>
    api.get(`/api/public/${shortCode}`),

  recordView: (shortCode: string) =>
    api.post(`/api/public/${shortCode}/view`),

  download: (shortCode: string) =>
    api.post(`/api/public/${shortCode}/download`),

  report: (shortCode: string, reason: string) =>
    api.post(`/api/public/${shortCode}/report`, { reason }),
};

// ─── Admin API ─────────────────────────────────────────────────────────────────
export const adminApi = {
  listMedia: (params?: {
    page?: number;
    page_size?: number;
    status?: string;
    type?: string;
    q?: string;
    user_id?: string;
    sort_by?: string;
    order?: string;
  }) => api.get("/api/admin/media", { params }),

  deleteMedia: (id: string) => api.delete(`/api/admin/media/${id}`),

  listUsers: (params?: { page?: number; page_size?: number; q?: string }) =>
    api.get("/api/admin/users", { params }),

  updateUser: (
    id: string,
    data: { is_active?: boolean; role?: string; storage_quota?: number }
  ) => api.patch(`/api/admin/users/${id}`, data),

  listReports: (params?: {
    page?: number;
    page_size?: number;
    status?: string;
  }) => api.get("/api/admin/reports", { params }),

  updateReport: (id: string, action: "resolve" | "dismiss") =>
    api.patch(`/api/admin/reports/${id}`, { action }),

  getStats: () => api.get("/api/admin/stats"),
};
