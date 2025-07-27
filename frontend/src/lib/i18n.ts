import { browser } from '$app/environment';
import { register, init, getLocaleFromNavigator } from 'svelte-i18n';

register('en', () => import('../locales/en.json'));
register('ru', () => import('../locales/ru.json'));

const defaultLocale = 'ru';

init({
  fallbackLocale: defaultLocale,
  initialLocale: browser ? (window.localStorage.getItem('locale') || getLocaleFromNavigator() || defaultLocale) : defaultLocale,
});