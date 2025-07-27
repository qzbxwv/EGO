<script lang="ts">
	import { locale } from 'svelte-i18n';

	function toggleLocale() {
		$locale = $locale === 'ru' ? 'en' : 'ru';
	}
</script>

<button
	onclick={toggleLocale}
	class="p-2 rounded-full hover:bg-tertiary transition-colors"
	title="Сменить язык / Change language"
>
	<div class="font-semibold w-5 h-5 flex items-center justify-center">
		{$locale === 'ru' ? 'EN' : 'RU'}
	</div>
</button>