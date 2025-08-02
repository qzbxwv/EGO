<script lang="ts">
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { toast } from 'svelte-sonner';
	import { Eye, EyeOff, User, Lock } from '@lucide/svelte';
	import GoogleLogin from '$lib/components/GoogleLogin.svelte';
	import { _ } from 'svelte-i18n';
	import LanguageSwitcher from '$lib/components/LanguageSwitcher.svelte';
	import { setAuthData } from '$lib/stores/auth.svelte.ts';
	import type { AuthResponse } from '$lib/types';
	import { LOGO_URL } from '$lib/config';

	let username = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let isLoading = $state(false);
	let showPassword = $state(false);

	async function handleRegister(event: SubmitEvent) {
		event.preventDefault();
		if (isLoading) return;
		if (password !== confirmPassword) {
			toast.error('Пароли не совпадают!');
			return;
		}
		isLoading = true;
		try {
			await api.post('/auth/register', { username, password });
			toast.success('Регистрация прошла успешно! Выполняется вход...');
			
			const loginResponse = await api.post<AuthResponse>('/auth/login', { username, password });

			if (loginResponse && loginResponse.access_token && loginResponse.refresh_token && loginResponse.user) {
				setAuthData(loginResponse.user, loginResponse.access_token, loginResponse.refresh_token);
				goto('/chat', { replaceState: true });
			} else {
				throw new Error('Не удалось автоматически войти. Попробуйте войти вручную.');
			}
		} catch (error: any) {
			toast.error(error.message || 'Ошибка регистрации. Возможно, такой пользователь уже существует.');
		} finally {
			isLoading = false;
		}
	}
</script>

<svelte:head>
	<title>{$_('register.register_button')} - EGO</title>
</svelte:head>

<div class="relative w-full h-full">
	<div class="absolute top-4 right-4 z-10">
		<LanguageSwitcher />
	</div>

	<div class="min-h-full flex items-center justify-center p-4">
		<div class="animated-border-card w-full max-w-md animate-fade-in-up">
			<div class="w-full bg-secondary p-5 sm:p-6 md:p-8 rounded-xl">
				<div class="text-center mb-4 md:mb-6">
					<img src={LOGO_URL} alt="EGO Logo" class="w-12 h-12 mx-auto mb-2" />
					<h1 class="text-2xl md:text-3xl font-bold text-text-primary">{$_('register.create_account')}</h1>
					<p class="text-text-secondary text-sm sm:text-base">{$_('register.prompt')}</p>
				</div>

				<form onsubmit={handleRegister} class="space-y-3 md:space-y-5">
					<div>
						<label for="username" class="block text-sm font-medium text-text-secondary mb-2">{$_('login.username')}</label>
						<div class="auth-input-wrapper">
							<input id="username" type="text" bind:value={username} required class="auth-input" placeholder={$_('register.username_placeholder')} />
							<User class="auth-input-icon w-5 h-5" />
						</div>
					</div>
					<div>
						<label for="password" class="block text-sm font-medium text-text-secondary mb-2">{$_('login.password')}</label>
						<div class="auth-input-wrapper relative">
							<input id="password" type={showPassword ? 'text' : 'password'} bind:value={password} required minlength="6" class="auth-input pr-10" placeholder="••••••••" />
							<Lock class="auth-input-icon w-5 h-5" />
							<button type="button" onclick={() => (showPassword = !showPassword)} class="absolute top-1/2 right-3 -translate-y-1/2 text-text-secondary hover:text-text-primary p-1 z-10" aria-label="Показать/скрыть пароль">
								{#if showPassword} <EyeOff class="w-5 h-5"/> {:else} <Eye class="w-5 h-5"/> {/if}
							</button>
						</div>
					</div>
					<div>
						<label for="confirm-password" class="block text-sm font-medium text-text-secondary mb-2">{$_('register.confirm_password')}</label>
						<div class="auth-input-wrapper">
							<input id="confirm-password" type={showPassword ? 'text' : 'password'} bind:value={confirmPassword} required class="auth-input" placeholder="••••••••" />
							<Lock class="auth-input-icon w-5 h-5" />
						</div>
					</div>
					<button type="submit" disabled={isLoading} class="w-full py-3 btn-gradient disabled:opacity-50 disabled:cursor-not-allowed !mt-6 hover:scale-105 active:scale-100">
						{isLoading ? $_('register.loading_button') : $_('register.register_button')}
					</button>
				</form>
				
				<div class="flex items-center my-3 md:my-5">
					<div class="flex-grow border-t border-tertiary"></div>
					<span class="flex-shrink mx-4 text-text-secondary text-xs uppercase">{$_('login.or')}</span>
					<div class="flex-grow border-t border-tertiary"></div>
				</div>

				<GoogleLogin />

				<div class="mt-5 text-center">
					<p class="text-sm text-text-secondary">
						{$_('register.have_account')}
						<a href="/login" class="font-semibold text-accent hover:text-accent-hover hover:underline">{$_('register.login')}</a>
					</p>
				</div>
			</div>
		</div>
	</div>
</div>