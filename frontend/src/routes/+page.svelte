<script lang="ts">
    import { goto } from '$app/navigation';
	import { browser } from '$app/environment';

    $effect(() => {
        if(browser) {
            const token = localStorage.getItem('accessToken');
            if (token) {
                goto('/chat', { replaceState: true });
            } else {
                goto('/login', { replaceState: true });
            }
        }
    });
</script>

<div class="flex h-screen w-full items-center justify-center bg-primary">
    <p class="text-text-secondary animate-pulse">Перенаправление...</p>
</div>