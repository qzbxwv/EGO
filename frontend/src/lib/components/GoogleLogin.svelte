<script lang="ts">
	import { PUBLIC_GOOGLE_CLIENT_ID } from '$env/static/public';
	import { api } from '$lib/api';
	import { goto } from '$app/navigation';
	import { toast } from 'svelte-sonner';
	import { setAuthData } from '$lib/stores/auth.svelte';

	let isLoading = $state(false);

	async function handleGoogleCallback(response: any) {
		isLoading = true;
		try {
			const res = await api.post<any>('/auth/google', { token: response.credential });
			
			if (res && res.access_token && res.refresh_token && res.user) {
				setAuthData(res.user, res.access_token, res.refresh_token);
				toast.success('Успешный вход через Google!');
				await goto('/chat', { replaceState: true });
			} else {
				throw new Error('Сервер вернул неполные данные для входа.');
			}
		} catch (error: any) {
			toast.error(error.message || 'Ошибка входа через Google');
		} finally {
			isLoading = false;
		}
	}

	$effect(() => {
		const interval = setInterval(() => {
			if (window.google && window.google.accounts) {
				clearInterval(interval);
				
				window.google.accounts.id.initialize({
					client_id: PUBLIC_GOOGLE_CLIENT_ID,
					callback: handleGoogleCallback
				});
				
				const buttonElement = document.getElementById('google-login-button');
				if (buttonElement) {
					window.google.accounts.id.renderButton(
						buttonElement, 
						{
							theme: 'filled_black',
							size: 'large',
							type: 'standard',
							shape: 'pill',
							text: 'signin_with',
							logo_alignment: 'left'
						}
					);
				}
			}
		}, 100);

        return () => {
            clearInterval(interval);
        }
	});
</script>

<div class="relative w-full flex justify-center items-center min-h-[44px]">
	{#if isLoading}
		<div class="absolute inset-0 flex items-center justify-center animate-pulse text-text-secondary">Вход через Google...</div>
	{/if}
	<div id="google-login-button" class:opacity-0={isLoading}></div>
</div>