import { redirect } from '@sveltejs/kit';
import { browser } from '$app/environment';

export function load() {
  if (browser) {
    const token = localStorage.getItem('accessToken');
    if (token) {
      throw redirect(307, '/chat');
    }
  }
  throw redirect(307, '/login');
}