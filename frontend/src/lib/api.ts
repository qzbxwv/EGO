import { PUBLIC_EGO_BACKEND_URL } from '$env/static/public';
import { auth, clearAuthData, setAccessToken } from '$lib/stores/auth.svelte.ts';
import { browser } from '$app/environment';
import { toast } from 'svelte-sonner';

const BASE_URL = PUBLIC_EGO_BACKEND_URL || '';

let isRefreshing = false;
let failedQueue: { resolve: (value?: unknown) => void; reject: (reason?: any) => void }[] = [];

const processQueue = (error: any, token: string | null = null) => {
	failedQueue.forEach((prom) => {
		if (error) {
			prom.reject(error);
		} else {
			prom.resolve(token);
		}
	});
	failedQueue = [];
};

async function request(path: string, options: RequestInit = {}, customFetch?: typeof fetch) {
	const isAuthEndpoint = path.startsWith('/auth/login') || path.startsWith('/auth/register');

	const fetcher = customFetch || (browser ? window.fetch : fetch);
	const headers = new Headers(options.headers || {});
	const token = auth.accessToken;

	if (token) {
		headers.set('Authorization', `Bearer ${token}`);
	}
	if (!headers.has('Content-Type') && options.body) {
		headers.set('Content-Type', 'application/json');
	}

	const requestOptions: RequestInit = { ...options, headers };

	let response = await fetcher(`${BASE_URL}${path}`, requestOptions);

	if (response.status === 401 && !isAuthEndpoint) {
		if (isRefreshing) {
			return new Promise((resolve, reject) => {
				failedQueue.push({ resolve, reject });
			}).then(() => {
				const newHeaders = new Headers(requestOptions.headers);
				newHeaders.set('Authorization', `Bearer ${auth.accessToken}`);
				requestOptions.headers = newHeaders;
				return fetcher(`${BASE_URL}${path}`, requestOptions);
			});
		}

		isRefreshing = true;
		const localRefreshToken = auth.refreshToken;

		if (localRefreshToken) {
			try {
				const refreshResponse = await fetcher(`${BASE_URL}/auth/refresh`, {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ refresh_token: localRefreshToken })
				});

				if (!refreshResponse.ok) {
					throw new Error('Session expired.');
				}

				const { access_token: newAccessToken } = await refreshResponse.json();
				setAccessToken(newAccessToken);

				headers.set('Authorization', `Bearer ${newAccessToken}`);
				requestOptions.headers = headers;
				response = await fetcher(`${BASE_URL}${path}`, requestOptions);

				processQueue(null, newAccessToken);
			} catch (e) {
				processQueue(e, null);
				if (browser) {
					sessionStorage.clear();
					clearAuthData();
				}
				return Promise.reject(e);
			} finally {
				isRefreshing = false;
			}
		} else {
			if (browser) {
				sessionStorage.clear();
				clearAuthData();
			}
			isRefreshing = false; 
			return Promise.reject(new Error('No refresh token available.'));
		}
	}

	if (!response.ok) {
		try {
			const errorData = await response.json();
			const errorMessage = errorData.detail || errorData.message || `Ошибка сервера: ${response.status}`;
			throw new Error(errorMessage);
		} catch (e: any) {
			const errorMessage = e.message || response.statusText || `Ошибка сервера: ${response.status}`;
			throw new Error(errorMessage);
		}
	}

	return response;
}

export const api = {
	async get<T>(path: string, customFetch?: typeof fetch): Promise<T> {
		const response = await request(path, { method: 'GET' }, customFetch);
		return response.json();
	},

	async post<T>(path: string, data: any, customFetch?: typeof fetch): Promise<T> {
		const response = await request(path, {
			method: 'POST',
			body: JSON.stringify(data)
		}, customFetch);
		const text = await response.text();
		return text ? JSON.parse(text) : (null as T);
	},

	async patch<T>(path: string, data: any, customFetch?: typeof fetch): Promise<T> {
		const response = await request(path, {
			method: 'PATCH',
			body: JSON.stringify(data)
		}, customFetch);
		const text = await response.text();
		return text ? JSON.parse(text) : (null as T);
	},

	async delete(path: string, customFetch?: typeof fetch): Promise<void> {
		await request(path, { method: 'DELETE' }, customFetch);
	}
};