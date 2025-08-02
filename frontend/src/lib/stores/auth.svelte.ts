import { browser } from '$app/environment';
import { goto } from '$app/navigation';
import type { User } from '$lib/types';

let user = $state<User | null>(null);
let accessToken = $state<string | null>(null);
let refreshToken = $state<string | null>(null);

export const auth = {
	get user() { return user; },
	get accessToken() { return accessToken; },
	get refreshToken() { return refreshToken; }
};

export function setAuthData(userData: User, access: string, refresh: string) {
	if (browser) {
		localStorage.setItem('user', JSON.stringify(userData));
		localStorage.setItem('accessToken', access);
		localStorage.setItem('refreshToken', refresh);
	}
	user = userData;
	accessToken = access;
	refreshToken = refresh;
}

export function setAccessToken(token: string) {
    accessToken = token;
    if (browser) {
        localStorage.setItem('accessToken', token);
    }
}

export function clearAuthData() {
	user = null;
	accessToken = null;
	refreshToken = null;
	if (browser) {
		localStorage.removeItem('user');
		localStorage.removeItem('accessToken');
		localStorage.removeItem('refreshToken');
	}
}


export function logout() {
	clearAuthData();
	if (browser) {
		goto('/login', { replaceState: true });
	}
}


export function initAuthStore() {
	if (browser) {
		const token = localStorage.getItem('accessToken');
		const refresh = localStorage.getItem('refreshToken');
		const userData = localStorage.getItem('user');
		
		if (token && refresh && userData) {
			try {
				user = JSON.parse(userData);
				accessToken = token;
				refreshToken = refresh;
			} catch (e) {
				console.error("Ошибка парсинга данных пользователя из localStorage", e);
				clearAuthData();
			}
		}
	}
}