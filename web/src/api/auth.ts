import { api } from "./client";
import type { User } from "./client";

interface AuthResponse {
  user: User;
  token: string;
}

export const authApi = {
  me: () => api.get<{ user: User }>("/api/v1/auth/me").then((r) => r.user),
  login: (email: string, password: string) =>
    api.post<AuthResponse>("/api/v1/auth/login", { email, password }).then((r) => r.user),
  register: (email: string, password: string, name: string) =>
    api.post<AuthResponse>("/api/v1/auth/register", { email, password, name }).then((r) => r.user),
  logout: () => api.post<void>("/api/v1/auth/logout"),
};
