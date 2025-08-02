import { browser } from '$app/environment';
import { locale, init, waitLocale } from 'svelte-i18n';
import type { LayoutLoad } from './$types';
import '$lib/i18n';
import { redirect } from '@sveltejs/kit';
import { initAuthStore } from '$lib/stores/auth.svelte.ts';
import { initializeWebSocket } from '$lib/ws-client';
import { api } from '$lib/api';
import { toast } from 'svelte-sonner';
import { setInitialSessions } from '$lib/stores/sessions.svelte.ts';
import type { ChatSession } from '$lib/types';

import 'highlight.js/styles/atom-one-dark.css';
import '../app.css';

export const load: LayoutLoad = async ({ data, url }) => {
	if (browser) {
		const token = localStorage.getItem('accessToken');
		const pathname = url.pathname;
		const isProtectedRoute = !['/login', '/register', '/'].includes(pathname);

		if (isProtectedRoute) {
			if (!token) {
				throw redirect(307, '/login');
			}
			
			initAuthStore();

			try {
				const sessions = await api.get<ChatSession[]>('/sessions');
				
				setInitialSessions(sessions);
				initializeWebSocket();

			} catch (error) {
				console.error("Auth validation failed in layout, redirecting.", error);
				toast.error('Сессия истекла. Пожалуйста, войдите заново.');
				throw redirect(307, '/login');
			}
		} else {
            initAuthStore();
        }
	}

	const currentLocale = browser 
		? (window.localStorage.getItem('locale') || data.initialLocale) 
		: data.initialLocale;

	init({
		fallbackLocale: 'ru',
		initialLocale: currentLocale,
	});

	await waitLocale(currentLocale);

	return data;
};