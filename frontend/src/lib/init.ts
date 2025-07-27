import { browser } from '$app/environment';
import { init, locale, waitLocale } from 'svelte-i18n';
import '$lib/i18n';

export async function initialize() {
    const defaultLocale = 'ru';

    init({
        fallbackLocale: defaultLocale,
        initialLocale: browser ? (localStorage.getItem('locale') || defaultLocale) : defaultLocale
    });

    await waitLocale();

    if (browser) {
        locale.subscribe(value => {
            if(value) {
                localStorage.setItem('locale', value);
            }
        });
    }
}