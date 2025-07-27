<script lang="ts">
	import { marked } from 'marked';
	import hljs from 'highlight.js';
	import { toast } from 'svelte-sonner';

	let { text } = $props<{ text: string }>();
	let container: HTMLDivElement | undefined = $state();

	function getLanguage(className: string): string | null {
		const match = /language-(\w+)/.exec(className || '');
		return match ? match[1] : null;
	}

	function copyCode(event: MouseEvent) {
		const button = event.currentTarget as HTMLButtonElement;
		const pre = button.closest('pre');
		const codeElement = pre?.querySelector('code.hljs') as HTMLElement;
		
		if (codeElement?.innerText) {
			navigator.clipboard.writeText(codeElement.innerText).then(() => {
				const icon = button.querySelector('svg');
				const originalIconHTML = icon?.outerHTML;
				if (!icon || !originalIconHTML) return;

				button.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"></path></svg>';
				button.disabled = true;
				
				setTimeout(() => {
					button.innerHTML = originalIconHTML;
					button.disabled = false;
				}, 2000);
			}).catch(err => {
				toast.error('Не удалось скопировать код');
				console.error("Copy to clipboard failed: ", err);
			});
		}
	}

	async function renderContent() {
		if (!container) return;
		if (!text) {
			container.innerHTML = '';
			return;
		}
		
		container.innerHTML = await marked(text, { async: true });
		
		const codeBlocks = container.querySelectorAll('pre code');
		codeBlocks.forEach((block) => {
			const pre = block.parentElement as HTMLPreElement;
			if (pre && !pre.querySelector('.code-footer')) {
				hljs.highlightElement(block as HTMLElement);
				
				const footer = document.createElement('div');
				footer.className = 'code-footer';

				const langName = getLanguage(block.className);

				const leftPart = document.createElement('div');
				leftPart.className = 'footer-left';
				
				const copyButton = document.createElement('button');
				copyButton.title = 'Скопировать код';
				copyButton.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>';
				copyButton.onclick = copyCode;

				const warningSpan = document.createElement('span');
				warningSpan.innerText = 'Use code with caution.';

				leftPart.appendChild(copyButton);
				leftPart.appendChild(warningSpan);

				const rightPart = document.createElement('div');
				rightPart.className = 'footer-right';
				rightPart.innerText = langName || '';

				footer.appendChild(leftPart);
				footer.appendChild(rightPart);

				pre.appendChild(footer);
			}
		});

		if (window.renderMathInElement) {
			window.renderMathInElement(container, {
				delimiters: [
					{ left: '$$', right: '$$', display: true },
					{ left: '$', right: '$', display: false }
				],
				throwOnError: false
			});
		}
	}

	$effect(() => {
		renderContent();
	});
</script>

<div bind:this={container} class="prose prose-invert max-w-none"></div>