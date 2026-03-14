import { User } from "./types";

// In-memory access token store (never in localStorage - XSS risk)
let accessToken: string | null = null;
let currentUser: User | null = null;

export function getAccessToken(): string | null {
  return accessToken;
}

export function setAccessToken(token: string): void {
  accessToken = token;
}

export function clearAccessToken(): void {
  accessToken = null;
  currentUser = null;
}

export function getCurrentUser(): User | null {
  return currentUser;
}

export function setCurrentUser(user: User): void {
  currentUser = user;
}

export function isAuthenticated(): boolean {
  return accessToken !== null;
}

export function isAdmin(): boolean {
  return currentUser?.role === "admin";
}
