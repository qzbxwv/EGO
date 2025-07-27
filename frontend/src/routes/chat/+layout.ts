import { redirect } from '@sveltejs/kit';
import { browser } from '$app/environment';
import { auth } from '$lib/stores/auth.svelte.ts'; 
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async () => {
  if (browser) {
    if (!auth.user) { 
      throw redirect(307, '/login');
    }
  }
  return {};
};