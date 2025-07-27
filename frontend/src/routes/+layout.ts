import { browser } from '$app/environment';
import { locale, init, waitLocale } from 'svelte-i18n';
import type { LayoutLoad } from './$types';
import '$lib/i18n'; 

import 'highlight.js/styles/atom-one-dark.css';
import '../app.css';

export const load: LayoutLoad = async ({ data }) => {
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